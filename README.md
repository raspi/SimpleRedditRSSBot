# SimpleRedditRSSBot

![GitHub All Releases](https://img.shields.io/github/downloads/raspi/SimpleRedditRSSBot/total?style=for-the-badge)
![GitHub release (latest by date)](https://img.shields.io/github/v/release/raspi/SimpleRedditRSSBot?style=for-the-badge)
![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/raspi/SimpleRedditRSSBot?style=for-the-badge)


Simple Reddit RSS Bot for submitting RSS feed links to subreddit(s). It keeps cache of newest 10 000 links submitted in a cache file per subreddit. URLs in RSS feeds are not resubmitted to a subreddit if there are duplicates.

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

## Create a directory for the bot

```
$ mkdir redditrssbot
$ cd redditrssbot
```
Full path is now for example `/home/raspi/redditrssbot`

Now download the latest release to this directory.

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

## Setup automatic submits to reddit with SystemD

Rename `systemd.service.dist` to `redditbot.service`.

Rename `systemd.timer.dist` to `redditbot.timer`.

Configure [.service](https://www.freedesktop.org/software/systemd/man/systemd.service.html):
```ini
[Unit]
Description=Reddit RSS Bot Service

[Service]
Type=oneshot
# !! Change these:
WorkingDirectory=/home/raspi/redditbot
ExecStart=/home/raspi/redditbot/redditrssbot-x64

[Install]
WantedBy=timers.target
```
Enable the `.service` file in user mode:
```
$ systemctl --user enable redditbot.service
```

Configure [.timer](https://www.freedesktop.org/software/systemd/man/systemd.timer.html):

```ini
[Unit]
Description=Reddit RSS Bot Timer

[Timer]
# Wait after boot
OnBootSec=10min
# run every X duration
# Recommended 1 hour (1h)
# Reddit is very strict how often links should be 
# submitted so use at least one hour sleep time. 
# Otherwise the bot will be banned.
OnUnitActiveSec=1h
Unit=redditbot.service

[Install]
WantedBy=timers.target
```

Enable and start the `redditbot.timer` timer in user mode:

```
$ systemctl --user enable redditbot.timer
$ systemctl --user start redditbot.timer
```

Check that the timer is listed:
```
$ systemctl --user list-timers
```

Check the logs with `journalctl --user -xe`.

## Troubleshooting

### SystemD user timer doesn't work when I log out

You must enable linger to your user as root as follows:

```
$ sudo loginctl enable-linger <user>
```

For example:

```
$ sudo loginctl enable-linger raspi
```
