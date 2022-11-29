package cmd

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"log"
)

var packageLog logr.Logger

var rootCmd = &cobra.Command{
	Use:              "package-manager",
	Short:            "Amazon EKS Anywhere Package Manager",
	Long:             "Manage Kubernetes packages with EKS Anywhere Curated Packages",
	PersistentPreRun: rootPersistentPreRun,
}

func init() {
	rootCmd.PersistentFlags().IntP("verbosity", "v", 0, "Set the log level verbosity")
	if err := viper.BindPFlags(rootCmd.PersistentFlags()); err != nil {
		log.Fatalf("failed to bind flags for root: %v", err)
	}
}

func rootPersistentPreRun(_ *cobra.Command, _ []string) {
	level := viper.GetInt("verbosity")
	cfg := zap.NewDevelopmentConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapcore.Level(-1 * level))
	cfg.EncoderConfig.EncodeLevel = nil
	cfg.DisableCaller = true
	cfg.DisableStacktrace = false
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	zapLog, err := cfg.Build()
	if err != nil {
		log.Fatalf("Error initializing logging: %v", err)
	}
	packageLog = zapr.NewLogger(zapLog)
}

func Execute() error {
	return rootCmd.ExecuteContext(context.Background())
}
