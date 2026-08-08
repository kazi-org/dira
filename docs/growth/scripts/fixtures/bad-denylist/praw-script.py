# Bad fixture: proves check-drafts.mjs's deny-list scan catches a `praw`
# (Python Reddit API Wrapper) import. Never run this file.
import praw

reddit = praw.Reddit(client_id="x", client_secret="x", user_agent="x")
