package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/mmcdole/gofeed"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"os"
	"sort"
	"time"
)

var VERSION = `0.0.0`                       // Version (git tag)
var BUILD = `dev`                           // Build (git hash)
var BUILDDATE = `0000-00-00T00:00:00+00:00` // Build date (git commit)
var USER_AGENT = fmt.Sprintf(`unix:SimpleGoRedditRSSBot:v%v build %v by /u/raspi`, VERSION, BUILD)

const (
	OVERRIDE_SUBMITTED_CHECK = false // for debugging purposes
	CONFIG_FILE              = `config.json`
	FEEDS_FILE               = `feeds.json`
)

type Configuration struct {
	Username string `json:"user"`
	Password string `json:"pass"`
	ClientId string `json:"cid"`
	Secret   string `json:"secret"`
}

// Load configuration JSON file
func LoadConfig(fname string) Configuration {
	cfgdata, err := ioutil.ReadFile(fname)
	if err != nil {
		log.Fatalf(`couldn't open %v'`, fname)
		panic(err)
	}
	var cfg Configuration

	err = json.Unmarshal(cfgdata, &cfg)
	if err != nil {
		panic(err)
	}

	return cfg
}

// Load already submitted cache file
// map[URL]submit time
func LoadSubmitted(fname string) (sub map[string]time.Time) {
	sub = make(map[string]time.Time, 0)

	f, err := os.Open(fname)
	if err != nil {
		if os.IsNotExist(err) {
			// Create new file
			ftmp, err := os.Create(fname)
			if err != nil {
				panic(err)
			}
			defer ftmp.Close()

			return LoadSubmitted(fname)
		}

		panic(err)
	}

	defer f.Close()

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		sub[scanner.Text()] = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	return sub
}

func (c *Configuration) ValidateConfiguration() (err error) {
	if c.Secret == `` {
		return fmt.Errorf(`empty secret`)
	}

	if c.ClientId == `` {
		return fmt.Errorf(`empty client id`)
	}

	if c.Password == `` {
		return fmt.Errorf(`empty password`)
	}

	if c.Username == `` {
		return fmt.Errorf(`empty user name`)
	}

	return nil
}

// map[URL]submit time
func SaveSubmitted(fname string, submitSource map[string]time.Time) {
	// Order the URLs by published date
	type KeyValuePair struct {
		Key   string
		Value int64
	}

	var sortedPairs []KeyValuePair
	for k, v := range submitSource {
		sortedPairs = append(sortedPairs, KeyValuePair{Key: k, Value: v.Unix()})
	}

	// Free memory
	submitSource = nil

	sort.Slice(sortedPairs, func(i, j int) bool {
		return sortedPairs[i].Value > sortedPairs[j].Value
	})

	f, err := ioutil.TempFile(`.`, fname)
	if err != nil {
		panic(err)
	}

	// Only remember N latest URLs
	urlsToKeep := 10000

	// List URLs in date order
	for _, kv := range sortedPairs {
		if urlsToKeep == 0 {
			// Old URLs are dropped from cache
			break
		}

		f.WriteString(fmt.Sprintf("%v\n", kv.Key))
		urlsToKeep--
	}

	f.Close()

	os.Rename(fname, `submitted_old.txt`)
	os.Rename(f.Name(), fname)
	os.Remove(`submitted_old.txt`)
}

func main() {
	errlog := log.New(os.Stderr, ``, log.LstdFlags)

	configFileArg := flag.String(`config`, CONFIG_FILE, `JSON config file name which has client secrets generated at reddit`)
	feedFileArg := flag.String(`feed`, FEEDS_FILE, `RSS feed JSON file name`)

	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stdout, "Simple Reddit RSS feed bot %v build %v\n", VERSION, BUILD)
		_, _ = fmt.Fprintf(os.Stdout, "Homepage <URL: https://github.com/raspi/SimpleRedditRSSBot >\n")
		_, _ = fmt.Fprintf(os.Stdout, "\n")
		_, _ = fmt.Fprintf(os.Stdout, "(c) Pekka Järvinen 2018-\n")
		_, _ = fmt.Fprintln(os.Stdout, `Parameters:`)

		flag.VisitAll(func(f *flag.Flag) {
			_, _ = fmt.Fprintf(os.Stdout, "  -%s\n      %s (default: %q)\n", f.Name, f.Usage, f.DefValue)
		})
	}

	flag.Parse()

	log.Printf(`Loading config..`)
	cfg := LoadConfig(*configFileArg)
	err := cfg.ValidateConfiguration()
	if err != nil {
		panic(err)
	}

	log.Printf(`Loading feeds..`)
	feeds := LoadFeedConfig(*feedFileArg)
	err = feeds.ValidateFeedConfig()
	if err != nil {
		panic(err)
	}

	defaultSubReddit := feeds.Subreddit

	var collectedLinks []SubmitLink

	// Collect URLs from feed(s)
	for _, feedSource := range feeds.Feeds {
		subReddit := feedSource.Subreddit

		if subReddit == `` {
			subReddit = defaultSubReddit
		}

		// RSS HTTP client
		fp := gofeed.NewParser()

		feed, err := fp.ParseURL(feedSource.UrlAddress)
		if err != nil {
			errlog.Printf(`error: feed '%v' URL %v parse error: %v`, feedSource.Title, feedSource.UrlAddress, err)
			continue
		}

		for _, item := range feed.Items {
			link, err := url.Parse(item.Link)
			if err != nil {
				errlog.Printf(`error: parsing URL %v - %v`, item.Link, err)
				continue
			}

			// Do a DNS lookup if URL has broken address
			ips, err := net.LookupIP(link.Host)
			if err != nil {
				errlog.Printf(`error: DNS lookup %v - %v`, item.Link, err)
				continue
			}

			if len(ips) == 0 {
				// Broken domain without IP address(es)
				errlog.Printf(`error: couldn't resolve IP address for %v`, link.String())
				continue
			}

			sl := SubmitLink{
				Title:     item.Title,
				Url:       link.String(),
				SubReddit: subReddit,
				Published: *item.PublishedParsed,
			}

			collectedLinks = append(collectedLinks, sl)
		}
	}

	log.Printf(`Got %v URLs from feeds..`, len(collectedLinks))

	log.Printf(`Removing cached..`)
	var submitLinks []SubmitLink

	// Remove cached
	for _, link := range collectedLinks {
		log.Printf(`Loading submitted cache..`)
		submitted := LoadSubmitted(fmt.Sprintf(`%s.cache`, link.SubReddit))

		// Check local cache
		_, ok := submitted[link.Url]

		if ok && !OVERRIDE_SUBMITTED_CHECK {
			continue
		}

		submitLinks = append(submitLinks, link)
	}

	// Free memory
	collectedLinks = []SubmitLink{}

	log.Printf(`Got %v URLs for submitting..`, len(submitLinks))

	// Submit new links to Reddit
	redditClient := New(cfg.Username, cfg.Password, cfg.ClientId, cfg.Secret, USER_AGENT)

	if len(submitLinks) > 0 {
		// Log in for submitting
		log.Printf(`Logging in..`)
		err = redditClient.Login()
		if err != nil {
			errlog.Fatalf(`Login failed: %v`, err)
		}
	}

	for _, link := range submitLinks {
		log.Printf(`Submitting to %v: %v [%v] - %v`, link.SubReddit, link.Title, link.Published, link.Url)

		// Submit link
		err = redditClient.SubmitLink(link)
		if err != nil {
			serr, ok := err.(*ErrorSubmitExists)

			if ok {
				submitFile := fmt.Sprintf(`%s.cache`, link.SubReddit)

				errlog.Printf("Already submitted: %v - %#v", link.Url, serr)
				submitted := LoadSubmitted(submitFile)
				submitted[serr.link.Url] = serr.link.Published

				log.Printf(`Saving submitted cache..`)
				SaveSubmitted(submitFile, submitted)
			} else {
				log.Fatalf(`%v`, err)
			}
		}

		// Sleep so that API isn't overloaded and bot doesn't get banned
		time.Sleep(time.Second * 2)

	}

}
