package main

import (
	"github.com/spf13/viper"
)

type Config struct {
	MONGO_URI         string `mapstructure:"MONGO_URI"`
	ATLAS_MONGO_URI   string `mapstructure:"ATLAS_MONGO_URI"`
	ETSY_CLIENT_ID    string `mapstructure:"ETSY_CLIENT_ID"`
	ETSY_REDIRECT_URI string `mapstructure:"ETSY_REDIRECT_URI"`
	APP_ENV           string `mapstructure:"APP_ENV"`
	SHOP_NAME         string `mapstructure:"SHOP_NAME"`
}

func LoadConfig(path string) (config Config, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName("app")
	viper.SetConfigType("env")

	viper.AutomaticEnv()

	err = viper.ReadInConfig()
	if err != nil {
		return
	}

	err = viper.Unmarshal(&config)
	return
}
