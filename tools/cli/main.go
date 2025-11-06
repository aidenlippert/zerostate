package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	version = "v0.1.0"
	cfgFile string
)

var rootCmd = &cobra.Command{
	Use:   "zerostate",
	Short: "zerostate CLI - Hybrid P2P network for AI agents",
	Long: `zerostate is a command-line tool for interacting with the zerostate network.
	
Manage Agent Cards, discover peers, and interact with the DHT.`,
	Version: version,
}

func init() {
	cobra.OnInitialize(initConfig)
	
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.zerostate.yaml)")
	rootCmd.PersistentFlags().String("bootstrap", "", "bootstrap peer multiaddr")
	rootCmd.PersistentFlags().String("listen", "/ip4/0.0.0.0/udp/0/quic-v1", "listen address")
	
	viper.BindPFlag("bootstrap", rootCmd.PersistentFlags().Lookup("bootstrap"))
	viper.BindPFlag("listen", rootCmd.PersistentFlags().Lookup("listen"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".zerostate")
	}
	
	viper.AutomaticEnv()
	
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
