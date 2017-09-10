package main

import (
	_ "expvar"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/TV4/graceful"
	"github.com/alexcesaro/statsd"
	"github.com/frozzare/hellobot/bot"
	"github.com/getsentry/raven-go"
)

var (
	c      *statsd.Client
	bt     *bot.Bot
	logger *log.Logger
)

func init() {
	dsn := os.Getenv("RAVEN_DSN")
	if len(dsn) > 0 {
		raven.SetDSN(dsn)
	}
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		if c != nil {
			logger.Println("Hello requests increment")
			c.Increment("hello.requests")
		}

		if err := bt.SayHello(r); err != nil {
			logger.Println(err)
		} else if c != nil {
			logger.Println("GitHub comments increment")
			c.Increment("github.comments")
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

func main() {
	logger = log.New(os.Stderr, "[hellobot] ", log.LstdFlags)

	port := os.Getenv("PORT")
	if len(port) == 0 {
		logger.Fatal("PORT environment variable is required")
	}

	if s := os.Getenv("STATSD_URL"); len(s) != 0 {
		var err error
		s = strings.Replace(s, "statsd://", "", -1)
		c, err = statsd.New(
			statsd.Address(s),
			statsd.Prefix("hellobot"),
		)
		if err != nil {
			logger.Fatal(err)
		}
		defer c.Close()
	}

	cert := os.Getenv("CERT")
	if len(cert) == 0 {
		logger.Fatal("CERT environment variable is required")
	}

	appID := os.Getenv("APP_ID")
	if len(appID) == 0 {
		logger.Fatal("APP_ID environment variable is required")
	}

	id, err := strconv.Atoi(appID)
	if err != nil {
		logger.Fatal(err)
	}

	bt = bot.NewBot(id, cert)

	http.HandleFunc("/hello", raven.RecoveryHandler(helloHandler))
	http.Handle("/", http.FileServer(http.Dir("site/public")))

	logger.Printf("Listening on http://0.0.0.0%s\n", ":"+port)
	graceful.ListenAndServe(&http.Server{
		Addr:    ":" + port,
		Handler: http.DefaultServeMux,
	})
}
