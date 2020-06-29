# Schrödinger's Box

This is the backend server of Schrödinger's Box - an event sharing and
publicizing platform targeting students and event organizers within NUS.

This app is still in early stage of development. All APIs provided by this
backend server complies with [JSON:API Specification v1.0](https://jsonapi.org/format/1.0/).

[![Build Status](https://travis-ci.com/Schrodinger-Box/backend.svg?branch=master)](https://travis-ci.com/Schrodinger-Box/backend)

## API documentation
API specification for this app is written in [OpenAPI Specification (Version 3.0.3)](https://github.com/OAI/OpenAPI-Specification/blob/master/versions/3.0.3.md).
Same as this backend server, it is still in early stage.

You can view it in a user-friendly documentation UI [here at SwaggerHub](https://app.swaggerhub.com/apis/schrodinger-box/schrodinger-box/1.0.0).

Development logs / raw file can also be viewed at [this Github repository](https://github.com/Schrodinger-Box/api/blob/1.0/swagger.yaml).

## Installation
```$xslt
git clone https://github.com/Schrodinger-Box/backend.git
cd backend
go build
cp schrodinger-box.sample.yaml schrodinger-box.yaml
vim schrodinger-box.sample.yaml
./schrodinger-box
```
Note: You will need Go v1.14.x for this server and its dependencies to
build and run.

## Configuration
There are detailed explanation on each of the items in the config file.
Please go check it out.

## Main dependencies
- [Gin](https://github.com/gin-gonic/gin) - Fast and light-weight web framework
- [GORM](https://github.com/go-gorm/gorm) - ORM library for Go
- [openid.go](https://github.com/Schrodinger-Box/openid-go) - OpenID (NUS authentication) library
- [gormid](https://github.com/Schrodinger-Box/gormid) - OpenID nonce and discovery cache library
- [tgbotapi](https://github.com/go-telegram-bot-api/telegram-bot-api) - Go binding for Telegram Bot API
- [cron](https://github.com/robfig/cron) - Cron-like scheduler for Go
- [viper](https://github.com/spf13/viper) - Configuration loader
- ...