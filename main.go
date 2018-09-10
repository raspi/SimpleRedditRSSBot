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

type ErrorAPI struct {
	err string
	url string
	val string
}

func (e *ErrorAPI) Error() string {
	return fmt.Sprintf("submit error: %v URL: %v\n%v", e.err, e.url, e.val)
}

type ErrorSubmitExists struct {
	err  string
	link SubmitLink
}

func (e *ErrorSubmitExists) Error() string {
	return fmt.Sprintf(`submit error: %v URL: %v`, e.err, e.link.Url)
}

var VERSION = `0.0.0`
var BUILD = `dev`
var USER_AGENT = fmt.Sprintf(`unix:SimpleGoRedditRSSBot:v%v build %v by /u/raspi`, VERSION, BUILD)

const (
	OVERRIDE_SUBMITTED_CHECK = false // for debugging purposes
	CONFIG_FILE              = `config.json`
	CACHE_FILE               = `submitted.txt`
	FEEDS_FILE               = `feeds.json`
)

type Configuration struct {
	Username string `json:"user"`
	Password string `json:"pass"`
	ClientId string `json:"cid"`
	Secret   string `json:"secret"`
}

func LoadConfig() Configuration {
	cfgdata, err := ioutil.ReadFile(CONFIG_FILE)
	if err != nil {
		log.Fatalf(`couldn't open %v'`, CONFIG_FILE)
		panic(err)
	}
	var cfg Configuration

	err = json.Unmarshal(cfgdata, &cfg)
	if err != nil {
		panic(err)
	}

	return cfg
}

func LoadSubmitted() (sub map[string]time.Time) {
	sub = make(map[string]time.Time, 0)

	f, err := os.Open(CACHE_FILE)
	if err != nil {
		if os.IsNotExist(err) {
			// Create new file
			ftmp, err := os.Create(CACHE_FILE)
			if err != nil {
				panic(err)
			}
			defer ftmp.Close()

			return LoadSubmitted()
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

func SaveSubmitted(submitSource map[string]time.Time) {
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

	f, err := ioutil.TempFile(`.`, CACHE_FILE)
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

	os.Rename(CACHE_FILE, `submitted_old.txt`)
	os.Rename(f.Name(), CACHE_FILE)
	os.Remove(`submitted_old.txt`)
}

type SubmitLink struct {
	Title     string    // Title of post
	Url       string    // URL of post
	SubReddit string    // Subreddit name
	Published time.Time // Published date and time (used for cache)
}

func main() {

	errlog := log.New(os.Stderr, ``, log.LstdFlags)

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Simple Reddit RSS feed bot %v build %v\n", VERSION, BUILD)
		fmt.Fprintf(flag.CommandLine.Output(), "Homepage <URL: https://github.com/raspi/SimpleRedditRSSBot >\n")
		fmt.Fprintf(flag.CommandLine.Output(), "\n")
		fmt.Fprintf(flag.CommandLine.Output(), "(c) Pekka Järvinen 2018-\n")
	}

	flag.Parse()

	log.Printf(`Loading config..`)
	cfg := LoadConfig()
	err := cfg.ValidateConfiguration()
	if err != nil {
		panic(err)
	}

	log.Printf(`Loading feeds..`)
	feeds := LoadFeedConfig()
	err = feeds.ValidateFeedConfig()
	if err != nil {
		panic(err)
	}

	log.Printf(`Loading submitted cache..`)
	submitted := LoadSubmitted()

	defaultSubReddit := feeds.Subreddit

	var collectedLinks []SubmitLink

	// Collect URLs from feed(s)
	for _, feedSource := range feeds.Feeds {
		subReddit := feedSource.Subreddit

		if subReddit == `` {
			subReddit = defaultSubReddit
		}

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
				// Broken domain without IP addresses
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
		log.Printf(`Submitting %v [%v] - %v`, link.Title, link.Published, link.Url)

		// Submit link
		err = redditClient.SubmitLink(link)
		if err != nil {
			serr, ok := err.(*ErrorSubmitExists)

			if ok {
				errlog.Printf("Already submitted: %v - %#v", link.Url, serr)
				submitted[serr.link.Url] = serr.link.Published
			} else {
				log.Fatalf(`%v`, err)
			}
		}

		// Sleep so that API isn't overloaded and bot doesn't get banned
		time.Sleep(time.Second * 2)

	}

	log.Printf(`Saving submitted cache..`)
	SaveSubmitted(submitted)
}
