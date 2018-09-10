package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
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

		// Check that URL scheme is valid
		_, err := url.Parse(feed.UrlAddress)
		if err != nil {
			return fmt.Errorf(`error: parsing URL %v - %v`, feed.UrlAddress, err)
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
