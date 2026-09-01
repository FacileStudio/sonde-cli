package cmd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/sonde-cli/internal/api"
	"github.com/FacileStudio/sonde-cli/internal/ui"
)

var (
	flagKeyApp     string
	flagKeyPublic  bool
	flagKeyOrigins string
	flagKeyQuota   int
	flagKeyYes     bool
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage API keys",
	Long: `Manage secret backend and public browser API keys. Requires an admin session
or a token in SONDE_TOKEN.`,
}

var keysListCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.NoArgs,
	Short: "List registered API keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signalContext()
		defer stop()

		client, err := newClient(true)
		if err != nil {
			return err
		}

		keys, err := client.ListKeys(ctx, flagKeyApp)
		if err != nil {
			return wrapInterrupt(ctx, err)
		}

		if flagJSON {
			return ui.JSON(keys)
		}
		if len(keys) == 0 {
			ui.Step("no API keys found")
			return nil
		}

		table := ui.Table("ID", "APP", "KIND", "PREFIX", "STATUS", "QUOTA", "CREATED")
		for _, k := range keys {
			status := "active"
			if k.RevokedAt != nil {
				status = "revoked"
			}
			quota := "unlimited"
			if k.DailyQuota > 0 {
				quota = fmt.Sprintf("%d/day (%d used)", k.DailyQuota, k.UsedToday)
			}
			table.Row(
				strconv.FormatInt(k.ID, 10),
				k.App,
				k.Kind,
				k.Prefix,
				status,
				quota,
				k.CreatedAt.Format(time.RFC3339),
			)
		}
		return table.Flush()
	},
}

var keysCreateCmd = &cobra.Command{
	Use:   "create",
	Args:  cobra.NoArgs,
	Short: "Create a new API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(flagKeyApp) == "" {
			return errors.New("--app is required")
		}

		ctx, stop := signalContext()
		defer stop()

		client, err := newClient(true)
		if err != nil {
			return err
		}

		kind := "secret"
		if flagKeyPublic {
			kind = "public"
		}

		var origins []string
		if flagKeyOrigins != "" {
			for _, o := range strings.Split(flagKeyOrigins, ",") {
				trimmed := strings.TrimSpace(o)
				if trimmed != "" {
					origins = append(origins, trimmed)
				}
			}
		}

		resp, err := client.CreateKey(ctx, api.CreateKeyRequest{
			App:            flagKeyApp,
			Kind:           kind,
			AllowedOrigins: origins,
			DailyQuota:     flagKeyQuota,
		})
		if err != nil {
			return wrapInterrupt(ctx, err)
		}

		if flagJSON {
			return ui.JSON(resp)
		}

		ui.Success("created %s key for %s (id: %d)", resp.Key.Kind, resp.Key.App, resp.Key.ID)
		fmt.Println(resp.Token)
		ui.Hint("store this token securely, it will not be shown again")
		return nil
	},
}

var keysRevokeCmd = &cobra.Command{
	Use:   "revoke <id>",
	Args:  cobra.ExactArgs(1),
	Short: "Revoke an API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil || id <= 0 {
			return errors.New("key id must be a positive integer")
		}

		ctx, stop := signalContext()
		defer stop()

		client, err := newClient(true)
		if err != nil {
			return err
		}

		if err := client.RevokeKey(ctx, id); err != nil {
			return wrapInterrupt(ctx, err)
		}

		if flagJSON {
			return ui.JSON(struct {
				ID      int64 `json:"id"`
				Revoked bool  `json:"revoked"`
			}{id, true})
		}

		ui.Success("key %d revoked", id)
		return nil
	},
}

func init() {
	keysListCmd.Flags().StringVar(&flagKeyApp, "app", "", "Filter keys by application name")

	keysCreateCmd.Flags().StringVar(&flagKeyApp, "app", "", "Application name")
	keysCreateCmd.Flags().BoolVar(&flagKeyPublic, "public", false, "Create a public browser key instead of a secret key")
	keysCreateCmd.Flags().StringVar(&flagKeyOrigins, "origins", "", "Comma-separated allowed origins for public keys")
	keysCreateCmd.Flags().IntVar(&flagKeyQuota, "quota", 0, "Daily event quota limit for public keys")
	_ = keysCreateCmd.MarkFlagRequired("app")

	keysRevokeCmd.Flags().BoolVarP(&flagKeyYes, "yes", "y", false, "Confirm revocation without prompting")

	keysCmd.AddCommand(keysListCmd, keysCreateCmd, keysRevokeCmd)
	rootCmd.AddCommand(keysCmd)
}
