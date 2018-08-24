# SimpleRedditRSSBot
Simple Reddit RSS Bot for submitting RSS feed links to subreddit(s).

## Setup bot app config
Register a new user for your bot in Reddit. Verify email. Set proper details.
Create new app @ https://www.reddit.com/prefs/apps/ type is **script**. See Reddit documentation for details.

Client ID can be read below app's name for example:
```
MyBotAppExample
personal use script
sgdbfseyseghbgsef <- Client id
```
**Secret** is listed separately in the same app page. 

Rename `config.json.dist` to `config.json`.

```json
{
  "user": "reddit username you registered for this bot, for example my_simple_bot",
  "pass": "reddit password you registered this bot",
  "cid": "your bot's app client id from https://www.reddit.com/prefs/apps/",
  "secret": "your bot's app secret from https://www.reddit.com/prefs/apps/"
}
```


## Setup feed URLs
Rename `feeds.json.dist` to `feeds.json`.

```json
{
  "subreddit": "my_news", Default subreddit name
  "feeds": [
    {
      "subreddit": "", Subreddit to post to, uses default if empty
      "title": "news", Title for logs, not used in reddit side
      "prefix": "", Prefix for links (not implemented)
      "suffix": "", Suffix for links (not implemented)
      "fid": "", Flair ID for links (not implemented)
      "flair": "", Flair text for links (not implemented)
      "url": "" RSS URL
    },
    {
      "subreddit": "my_patch_news", Second feed, etc
      "title": "patches", Title for logs, not used in reddit side
      "prefix": "", Prefix for links (not implemented)
      "suffix": "", Suffix for links (not implemented)
      "fid": "", Flair ID for links (not implemented)
      "flair": "", Flair text for links (not implemented)
      "url": "" RSS URL
    }
  ]
}
```
