package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"text/template"
	"time"

	slackbot "github.com/dutchcoders/slackbot"
)

type XKCD struct {
	Alt        string `json:"alt"`
	Day        string `json:"day"`
	Img        string `json:"img"`
	Link       string `json:"link"`
	Month      string `json:"month"`
	News       string `json:"news"`
	Num        int    `json:"num"`
	SafeTitle  string `json:"safe_title"`
	Title      string `json:"title"`
	Transcript string `json:"transcript"`
	Year       string `json:"year"`
}

type Attachment struct {
	Fallback   string `json:"fallback"`
	Title      string `json:"title"`
	TitleLink  string `json:"title_link"`
	Text       string `json:"text"`
	AuthorName string `json:"author_name"`
	AuthorLink string `json:"author_link"`
	ImageUrl   string `json:"image_url"`
}

type Payload struct {
	Username    string       `json:"username,omitempty"`
	Text        string       `json:"text,omitempty"`
	Channel     string       `json:"channel,omitempty"`
	Attachments []Attachment `json:"attachments"`
}

var ErrTimeout = errors.New("Timeout occured")

func get(num int) (*XKCD, error) {
	incoming := make(chan XKCD)
	err_chan := make(chan error)

	url := fmt.Sprintf("http://xkcd.com/%d/info.0.json", num)

	if num == 0 {
		url = fmt.Sprintf("http://xkcd.com/info.0.json")
	}

	go func() {
		var err error
		var resp *http.Response
		if resp, err = http.Get(url); err != nil {
			err_chan <- err
			return
		}

		var xkcd XKCD
		if err := json.NewDecoder(resp.Body).Decode(&xkcd); err != nil {
			err_chan <- err
			return
		}

		incoming <- xkcd
		return
	}()

	select {
	case <-time.After(time.Second * 2):
		return nil, ErrTimeout
	case err := <-err_chan:
		return nil, err
	case xkcd := <-incoming:
		return &xkcd, nil
	}
}

func comic(num int, sc *slackbot.Context, w http.ResponseWriter) {
	xkcd, err := get(num)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	webhook_url := os.Getenv("WEBHOOK_URL")

	payload := Payload{
		Username: "xkcd",
		Text:     fmt.Sprintf("A webcomic of romance, sarcasm, math, and language. (%d)", xkcd.Num),
		Channel:  sc.ChannelID,
		Attachments: []Attachment{
			Attachment{
				Fallback:   xkcd.Alt,
				AuthorName: "XKCD",
				AuthorLink: "http://xkcd.com",
				Title:      xkcd.Title,
				TitleLink:  fmt.Sprintf("http://xkcd.com/%d/", xkcd.Num),
				Text:       xkcd.Alt,
				ImageUrl:   xkcd.Img,
			},
		},
	}

	buffer := new(bytes.Buffer)
	if err := json.NewEncoder(buffer).Encode(payload); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	client := &http.Client{}

	var req *http.Request
	if req, err = http.NewRequest("POST", webhook_url, buffer); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	if _, err := client.Do(req); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}

func help(sc *slackbot.Context, w http.ResponseWriter) {
	var err error

	t := template.New("")
	t, err = t.Parse(`*Usage*:
` + "```" + `
/xkcd latest
/xkcd random 
/xkcd [num] 
` + "```" + `
`)
	if err != nil {
		log.Printf("Error templating results %#v\n", err)
		return
	}

	err = t.Execute(w, nil)
	if err != nil {
		log.Printf("Error executing template %#v\n", err)
		return
	}

}

func xkcd(sc *slackbot.Context, w http.ResponseWriter) {
	r := regexp.MustCompile(`(random|latest|help)?\s*(.*)`)

	matches := r.FindAllStringSubmatch(sc.Text, -1)

	if len(matches) == 0 {
		return
	}

	sc.Text = matches[0][2]

	fmt.Println(matches[0][2])
	if num, err := strconv.Atoi(matches[0][2]); err == nil {
		comic(num, sc, w)
		return
	}

	switch matches[0][1] {
	case "":
		help(sc, w)
	case "random":
		num := rand.Intn(cache.Latest())
		comic(num, sc, w)
	case "latest":
		comic(0, sc, w)
	default:
	}

}
