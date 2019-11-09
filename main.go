package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/go-redis/redis"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/iph0/conf"
	"github.com/iph0/conf/envconf"
	"github.com/iph0/conf/fileconf"
	"go.uber.org/zap"
	"golang.org/x/net/proxy"
)

type redisConf struct {
	Addrs string
}

type tgConf struct {
	Token string
	URL   string
	Path  string
	Proxy string
}

type beckaConf struct {
	Telegram tgConf
	Redis    redisConf
}

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	log := logger.Sugar()

	fileLdr := fileconf.NewLoader("etc", "/etc")
	envLdr := envconf.NewLoader()

	configProc := conf.NewProcessor(
		conf.ProcessorConfig{
			Loaders: map[string]conf.Loader{
				"file": fileLdr,
				"env":  envLdr,
			},
		},
	)

	configRaw, err := configProc.Load(
		"file:becka.yml",
		"env:^BECKA_",
	)

	if err != nil {
		log.Panic(err)
	}

	var cnf beckaConf
	if err := conf.Decode(configRaw["becka"], &cnf); err != nil {
		log.Panic(err)
	}

	addrs := strings.Split(cnf.Redis.Addrs, ",")

	ropt := &redis.ClusterOptions{
		Addrs: addrs,
	}

	rDB := redis.NewClusterClient(ropt)

	httpTransport := &http.Transport{}

	if len(cnf.Telegram.Proxy) > 0 {
		dialer, err := proxy.SOCKS5("tcp", cnf.Telegram.Proxy, nil, proxy.Direct)
		if err != nil {
			log.Panic(err)
		}

		httpTransport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			done := make(chan bool)
			var con net.Conn
			var err error

			go func() {
				con, err = dialer.Dial(network, addr)
				done <- true
			}()

			select {
			case <-ctx.Done():
				return nil, errors.New("Dial timeout")
			case <-done:
				return con, err
			}
		}
	}

	httpClient := &http.Client{Transport: httpTransport}

	bot, err := tgbotapi.NewBotAPIWithClient(cnf.Telegram.Token, httpClient)
	if err != nil {
		log.Panic(err)
	}

	res, err := bot.SetWebhook(tgbotapi.NewWebhook(cnf.Telegram.URL + cnf.Telegram.Path))
	if err != nil {
		log.Panic(err)
	}

	log.Debug(res.Description)

	updates := bot.ListenForWebhook("/" + cnf.Telegram.Path)

	http.HandleFunc("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	})

	go func() {
		if err := http.ListenAndServe("0.0.0.0:8080", nil); err != nil {
			log.Panic(err)
		}
	}()

	go func() {
		for upd := range updates {
			if upd.Message == nil {
				continue
			}

			if upd.Message.Sticker != nil {
				key := fmt.Sprintf("becka{%d}", upd.Message.From.ID)
				res, err := rDB.Incr(key).Result()
				if err != nil {
					continue
				}

				if res == 1 {
					err = rDB.Expire(key, time.Hour*24).Err()
					if err != nil {
						continue
					}
				}

				if res > 10 {
					log.Debugf("Delete %s for %d", upd.Message.From.UserName, res)

					_, err := bot.DeleteMessage(tgbotapi.DeleteMessageConfig{
						ChatID:    upd.Message.Chat.ID,
						MessageID: upd.Message.MessageID,
					})

					if err != nil {
						log.Error(err)
					}
				}
			}
		}
	}()

	st := make(chan os.Signal, 1)
	signal.Notify(st, os.Interrupt)

	<-st
	log.Info("Stop")

	_ = log.Sync()
}
