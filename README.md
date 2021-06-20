# flashpoint-submission-system

it's a thing, and it's pretty WIP

it will be a submission management system for https://bluemaxima.org/flashpoint/ or something

<img src="static/opal.png" alt="drawing" width="200"/>

## How to run this thing
- it's using discord for user auth, so you need a discord app with oauth config
- set up a discord bot to read user roles (FYI roles are hardcoded for Flashpoint discord server), roles are used for permission inside the system
- set up a discord bot to post notifications, can be the same bot as the previous one
- start a mysql instance, `make db` will do the work for you if you're a fan docker-compose
- start a curation validator server https://github.com/FlashpointProject/Curation-Validation-Bot
- fill in all the stuff in .env
- start the thing using `go run ./main/*.go`

it looks something like this

![submit page](github/ss2.png)

and this

![submissions page](github/ss3.png)

and this

![submission page](github/ss4.png)

and also this

![profile page](github/ss1.png)