package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type RedditSubmitErrorJson struct {
	JQuery [][]interface{} `json:"jquery,omitempty"`
	JSON   struct {
		Errors [][]string `json:"errors,omitempty"`
		Data   struct {
			Url         string `json:"url,omitempty"`
			Id          string `json:"id,omitempty"`
			Name        string `json:"name,omitempty"`
			DraftsCount uint64 `json:"drafts_count,omitempty"`
		} `json:"data,omitempty"`
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

func (r *Reddit) SubmitLink(link SubmitLink) error {
	v := url.Values{}
	v.Set("sr", link.SubReddit)
	v.Set("title", link.Title)
	v.Set("url", link.Url)
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

	// Check content type
	ctype := resp.Header.Get("Content-Type")
	if !strings.Contains(ctype, `application/json`) {
		return fmt.Errorf(`invalid content type: %v html: %v`, ctype, string(htmlData))
	}

	if resp.StatusCode != http.StatusOK {
		return &ErrorAPI{
			err: string(htmlData),
			url: req.URL.RequestURI(),
			val: v.Encode(),
		}
	}

	log.Printf(`%v`, string(htmlData))

	// Convert JSON to struct
	var tmp RedditSubmitErrorJson
	err = json.Unmarshal(htmlData, &tmp)
	if err != nil {
		return fmt.Errorf(`error: %v`, string(htmlData))
	}

	var errs []string

	if len(tmp.JSON.Errors) > 0 {
		for _, item := range tmp.JSON.Errors[0] {
			if item == `ALREADY_SUB` {
				return &ErrorSubmitExists{
					err:  `link already submitted`,
					link: link,
				}
			}

			errs = append(errs, item)
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf(`%v`, strings.Join(errs, ". "))
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
	err  string
	link SubmitLink
}

func (e *ErrorSubmitExists) Error() string {
	return fmt.Sprintf(`submit error: %v URL: %v`, e.err, e.link.Url)
}

// Submit link information
type SubmitLink struct {
	Title     string    // Title of post
	Url       string    // URL of post
	SubReddit string    // Subreddit name
	Published time.Time // Published date and time (used for cache)
}
