package main

import (
	"log"

	"github.com/cloudfoundry-community/go-cfenv"
	"github.com/kelseyhightower/envconfig"
)

type appConfig struct {
	IpaHost     string `required:"true"`
	IpaUser     string `required:"true"`
	IpaPassword string `required:"true"`

	EmailHost     string `required:"true"`
	EmailPort     int    `required:"true"`
	EmailFrom     string `required:"true"`
	EmailUser     string
	EmailPassword string

	RedisHost     string
	RedisPort     int    `default:"6379"`
	RedisPassword string `default:""`
	RedisDB       int    `default:"0"`

	AppName string `default:"IPA Password Reset Selfservice"`
	AppPort int    `default:"9000"`

	TokenValidity int `default:"5"`
}

func LoadConfig() appConfig {
	var config appConfig

	err := envconfig.Process("pwreset", &config)
	if err != nil {
		log.Fatal(err)
	}

	if cfenv.IsRunningOnCF() {
		appEnv, _ := cfenv.Current()
		config.AppPort = appEnv.Port

		redisServices, err := appEnv.Services.WithTag("redis")
		if err != nil {
			log.Fatal(err)
		}
		config.RedisHost = redisServices[0].Credentials["host"].(string)
		config.RedisPort = int(redisServices[0].Credentials["port"].(float64))
		config.RedisPassword = redisServices[0].Credentials["password"].(string)
	}
	return config
}
