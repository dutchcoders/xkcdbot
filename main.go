package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"text/template"
	"time"

	slackbot "github.com/dutchcoders/slackbot"
	"github.com/gorilla/mux"
	"github.com/nlopes/slack"
)

var api *slack.Slack
var config Config
var templates = template.Must(template.ParseGlob("static/*.html"))

func init() {
	api = slack.New(os.Getenv("SLACK_TOKEN"))

	rand.Seed(time.Now().UTC().UnixNano())
}

type Cache struct {
	mutex  sync.Mutex
	latest int
}

func (c *Cache) lock() {
	c.mutex.Lock()
}

func (c *Cache) unlock() {
	c.mutex.Unlock()
}

func (c *Cache) Latest() int {
	return c.latest
}

func (c *Cache) SetLatest(num int) {
	c.latest = num
}

var cache Cache = NewCache()

func NewCache() Cache {
	return Cache{mutex: sync.Mutex{}, latest: 1500}
}

func HelloServer(w http.ResponseWriter, req *http.Request) {
	io.WriteString(w, "hello, world!\n")
}

func notFoundHandler(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(w, "Not found.")
}

func PageHandler(page string) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		if err := templates.ExecuteTemplate(w, page, nil); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func main() {
	go func() {
		// cache updater
		for {
			log.Printf("Updating cache...")

			if xkcd, err := get(0); err == nil {
				log.Printf("Updated latest comic to %d.\n", xkcd.Num)
				cache.SetLatest(xkcd.Num)
			}

			time.Sleep(time.Minute * 5)
		}
	}()

	/*
		go func() {
			bot, err := slackbot.NewBot(slackbot.Config{
				Token:  os.Getenv("SLACK_TOKEN"),
				Origin: "http://localhost",
			})

			if err != nil {
				log.Println(err)
				return
			}

			bot.SetMessageHandler(func(b *slackbot.Bot, message *slackbot.Message) error {
				//	reply := b.NewMessage()
				//	reply.Channel = message.Channel
				//	reply.Text = message.Text
				//	b.Send(reply)
				return nil
			})

			err = bot.Run()
			if err != nil {
				log.Println(err)
			}
		}()
	*/

	r := mux.NewRouter()

	engine := slackbot.NewEngine(slackbot.Config{
		PayloadToken: os.Getenv("SLACK_PAYLOAD_TOKEN"),
	})

	engine.AddCommand("/xkcd", xkcd)

	r.Methods("POST").Handler(engine)

	r.HandleFunc("/", PageHandler("index.html"))
	r.PathPrefix("/img/").Handler(http.FileServer(http.Dir("static/")))
	r.NotFoundHandler = http.HandlerFunc(notFoundHandler)

	addr := ":" + os.Getenv("PORT")
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("ListenAndServe %s: %v", addr, err)
	}

}

func find(users []slack.User, name string) (*slack.User, error) {
	for _, user := range users {
		switch {
		case strings.EqualFold(user.Name, name):
		case strings.EqualFold(user.Profile.RealName, name):
		case strings.EqualFold(user.Profile.RealNameNormalized, name):

		default:
			continue
		}

		return &user, nil
	}

	return nil, errors.New("Not found")
}
