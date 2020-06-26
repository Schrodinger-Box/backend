package misc

import "github.com/spf13/viper"

func APIAbsolutePath(relativePath string) string {
	return viper.GetString("domain") + viper.GetString("apiRoot") + relativePath
}
