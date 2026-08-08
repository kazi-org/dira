# Bad fixture: proves check-drafts.mjs's deny-list scan catches a `tweepy`
# import. Never run this file.
import tweepy

client = tweepy.Client(bearer_token="x")
