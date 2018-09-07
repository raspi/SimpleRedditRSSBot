package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/mmcdole/gofeed"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

type FeedConfig struct {
	Subreddit string `json:"subreddit"`
	Feeds     []struct {
		Subreddit  string `json:"subreddit,omitempty"`
		Title      string `json:"title,omitempty"`
		Prefix     string `json:"prefix,omitempty"`
		Suffix     string `json:"suffix,omitempty"`
		FlairId    string `json:"fid,omitempty"`
		Flair      string `json:"flair,omitempty"`
		UrlAddress string `json:"url"`
	} `json:"feeds"`
}

type ErrorAPI struct {
	err string
	url string
	val string
}

func (e *ErrorAPI) Error() string {
	return fmt.Sprintf("submit error: %v URL: %v\n%v", e.err, e.url, e.val)
}

type ErrorSubmitExists struct {
	err string
	url string
}

func (e *ErrorSubmitExists) Error() string {
	return fmt.Sprintf(`submit error: %v URL: %v`, e.err, e.url)
}

type RedditSubmitErrorJson struct {
	JQuery [][]interface{} `json:"jquery,omitempty"`
	JSON   struct {
		Errors [][]string `json:"errors,omitempty"`
	} `json:"json,omitempty"`
}

type RedditAccessToken struct {
	Id           string
	Type         string
	RefreshToken string
	ExpiresIn    time.Time
}

type RedditAccessTokenJson struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	Scope       string `json:"scope"`
	Error       string `json:"error,omitempty"`
}

type Reddit struct {
	Client    *http.Client
	Id        string
	Secret    string
	UserAgent string
	Scopes    []string
	Username  string
	Password  string
	Uri       string
	Rate      time.Duration
	State     string
	Token     RedditAccessToken
	limiter   <-chan time.Time
}

func New(username, password, id, secret string, userAgent string) Reddit {

	dur := time.Second

	limiter := time.Tick(dur)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
		},
	}
	client.Timeout = time.Second * 15

	return Reddit{
		Client:    client,
		Rate:      dur,
		Id:        id,
		Secret:    secret,
		Username:  username,
		Password:  password,
		UserAgent: userAgent,
		limiter:   limiter,
		Token: RedditAccessToken{
			Id:        "",
			ExpiresIn: time.Now(),
		},
	}
}

// Log in to Reddit
func (r *Reddit) Login() (err error) {
	v := url.Values{}
	v.Set("grant_type", "password")
	v.Set("username", r.Username)
	v.Set("password", r.Password)

	req, err := http.NewRequest("POST", "https://www.reddit.com/api/v1/access_token", strings.NewReader(v.Encode()))

	if err != nil {
		log.Println(err)
		return fmt.Errorf(``)
	}
	req.SetBasicAuth(r.Id, r.Secret)
	req.Header.Add("User-Agent", r.UserAgent)

	resp, err := r.Client.Do(req)

	if err != nil {
		log.Println(err)
		return fmt.Errorf(`request error`)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(`status code %v`, resp.StatusCode)
	}

	htmlData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return fmt.Errorf(`read error`)
	}
	defer resp.Body.Close()

	// Check content type
	ctype := resp.Header.Get("Content-Type")
	if !strings.Contains(ctype, `application/json`) {
		return fmt.Errorf(`invalid content type: %v html: %v`, ctype, string(htmlData))
	}

	// JSON to struct
	var tmp RedditAccessTokenJson

	err = json.Unmarshal(htmlData, &tmp)
	if err != nil {
		return fmt.Errorf(`request error: %v`, err)
	}

	if tmp.Error != "" {
		return fmt.Errorf(`login error: %v`, tmp.Error)
	}

	// Generate token
	r.Token = RedditAccessToken{
		Id:        tmp.AccessToken,
		ExpiresIn: time.Now().Add(time.Duration(tmp.ExpiresIn) * time.Second),
		Type:      tmp.TokenType,
	}

	return nil

}

func (r *Reddit) SubmitLink(subReddit string, title string, link string) error {
	v := url.Values{}
	v.Set("sr", subReddit)
	v.Set("title", title)
	v.Set("url", link)
	v.Set("kind", "link")
	v.Set("uh", "")
	//v.Set("flair_text", flair)
	v.Set("resubmit", "false")
	//v.Set("ad", "false")
	v.Set("nsfw", "false")
	//v.Set("spoiler", "false")
	v.Set("api_type", "json")

	req, err := http.NewRequest("POST", "https://oauth.reddit.com/api/submit", strings.NewReader(v.Encode()))

	if err != nil {
		log.Println(err)
		return fmt.Errorf(`error building request`)
	}
	req.Header.Add("User-Agent", r.UserAgent)
	req.Header.Add("Authorization", fmt.Sprintf(`%v %v`, r.Token.Type, r.Token.Id))

	resp, err := r.Client.Do(req)

	if err != nil {
		log.Println(err)
		return fmt.Errorf(`request error`)
	}

	htmlData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return fmt.Errorf(`read error`)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &ErrorAPI{
			err: string(htmlData),
			url: req.URL.RequestURI(),
			val: v.Encode(),
		}
	}

	log.Printf(`%v`, string(htmlData))

	var tmp RedditSubmitErrorJson
	err = json.Unmarshal(htmlData, &tmp)
	if err != nil {
		return fmt.Errorf(`error: %v`, string(htmlData))
	}

	var errs []string

	if len(tmp.JSON.Errors) > 0 {
		for _, item := range tmp.JSON.Errors[0] {
			if item == `ALREADY_SUB` {
				return &ErrorSubmitExists{err: `link already submitted`, url: link}
			}

			errs = append(errs, item)
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf(`%v`, strings.Join(errs, ". "))
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

func LoadFeedConfig() FeedConfig {
	cfgdata, err := ioutil.ReadFile(FEEDS_FILE)
	if err != nil {
		log.Fatalf(`couldn't open %v'`, FEEDS_FILE)
	}

	var cfg FeedConfig

	err = json.Unmarshal(cfgdata, &cfg)
	if err != nil {
		panic(err)
	}

	return cfg
}

func (c *FeedConfig) ValidateFeedConfig() (err error) {

	if c.Subreddit == `` {
		return fmt.Errorf(`default subreddit empty`)
	}

	seenTitles := make(map[string]bool)

	seenUrls := make(map[string]bool)

	for _, feed := range c.Feeds {
		if feed.UrlAddress == `` {
			return fmt.Errorf(`empty URL address`)
		}

		if feed.Title == `` {
			return fmt.Errorf(`empty title for %v`, feed.UrlAddress)
		}

		_, urlOk := seenUrls[feed.UrlAddress]

		if !urlOk {
			seenUrls[feed.UrlAddress] = true
		} else {
			return fmt.Errorf(`URL address %v exists already`, feed.UrlAddress)
		}

		_, titleOk := seenTitles[feed.Title]

		if !titleOk {
			seenTitles[feed.Title] = true
		} else {
			return fmt.Errorf(`title %v exists already ref: %v`, feed.Title, feed.UrlAddress)
		}

	}

	return nil
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

func main() {

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

	log.Printf(`Loading submitted..`)
	submitted := LoadSubmitted()

	loggedIn := false

	defaultSubReddit := feeds.Subreddit

	r := Reddit{}

	for _, feedSource := range feeds.Feeds {
		subReddit := feedSource.Subreddit

		if subReddit == `` {
			subReddit = defaultSubReddit
		}

		fp := gofeed.NewParser()

		feed, _ := fp.ParseURL(feedSource.UrlAddress)
		log.Printf(`Feed: %v | %v %v %v`, feedSource.Title, feed.Title, feed.Description, feed.Link)

		for _, item := range feed.Items {
			log.Printf(`  >> %v [%v]`, item.Title, item.PublishedParsed)

			_, ok := submitted[item.Link]

			if ok && !OVERRIDE_SUBMITTED_CHECK {
				log.Printf(`    Found in local cache, skipping`)
				continue
			}

			if !loggedIn {
				log.Printf(`Logging in..`)
				r = New(cfg.Username, cfg.Password, cfg.ClientId, cfg.Secret, USER_AGENT)
				err = r.Login()
				if err != nil {
					log.Fatalf(`Login failed: %v`, err)
				}

				loggedIn = true
			}

			err = r.SubmitLink(subReddit, item.Title, item.Link)
			if err != nil {
				serr, ok := err.(*ErrorSubmitExists)

				if ok {
					log.Printf("    Already submitted:\n  %v", item.Link)
					submitted[serr.url] = *item.PublishedParsed
				} else {
					log.Fatalf(`%v`, err)
				}
			}

			time.Sleep(time.Second * 2)
		}
	}

	log.Printf(`Saving submitted..`)
	SaveSubmitted(submitted)
}
