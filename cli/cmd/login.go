package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const clientID = "spade-cli"

type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type tokenErrorResponse struct {
	Error string `json:"error"`
}

var loginServerURL string

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate to the cloud registry",
	Long: `Authenticate the Spade CLI using the OAuth 2.0 device authorization flow
(RFC 8628). A short code is displayed in the terminal — open the verification
URL in a browser, enter the code, and approve access.

Credentials are stored in ~/.spade/auth/ and used by spade publish and
other authenticated commands. Run 'spade logout' to remove them.

The server URL is resolved in this order:
  1. --server flag
  2. SPADE_SERVER environment variable
  3. 'server' key in ~/.spade.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		serverURL := loginServerURL
		if serverURL == "" {
			serverURL = os.Getenv("SPADE_SERVER")
		}
		if serverURL == "" {
			serverURL = viper.GetString("server")
		}
		if serverURL == "" {
			return fmt.Errorf("server URL required: set --server, SPADE_SERVER, or 'server' in ~/.spade.yaml")
		}
		return runLogin(http.DefaultClient, serverURL)
	},
}

func init() {
	loginCmd.Flags().StringVar(&loginServerURL, "server", "", "Spade server URL")
	rootCmd.AddCommand(loginCmd)
}

func runLogin(client *http.Client, serverURL string) error {
	dc, err := requestDeviceCode(client, serverURL)
	if err != nil {
		return fmt.Errorf("requesting device code: %w", err)
	}

	interval := dc.Interval
	if interval <= 0 {
		interval = 5
	}

	fmt.Printf("\nTo authorize this device:\n\n")
	fmt.Printf("  1. Open:  %s\n", dc.VerificationURI)
	fmt.Printf("  2. Enter: %s\n\n", dc.UserCode)

	token, err := pollForToken(client, serverURL, dc.DeviceCode, interval)
	if err != nil {
		return err
	}

	if err := SaveCredentials(Credentials{
		Token:     token,
		ServerURL: serverURL,
		IssuedAt:  time.Now(),
	}); err != nil {
		return fmt.Errorf("saving credentials: %w", err)
	}

	fmt.Println("Logged in successfully.")
	return nil
}

func requestDeviceCode(client *http.Client, serverURL string) (deviceCodeResponse, error) {
	body, _ := json.Marshal(map[string]string{"client_id": clientID})
	resp, err := client.Post(
		serverURL+"/api/auth/device/code",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return deviceCodeResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return deviceCodeResponse{}, fmt.Errorf("server returned %d: %s", resp.StatusCode, b)
	}

	var dc deviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dc); err != nil {
		return deviceCodeResponse{}, fmt.Errorf("decoding response: %w", err)
	}
	return dc, nil
}

// doTokenRequest makes a single POST to the token endpoint.
// Returns (token, "", nil) on success.
// Returns ("", errCode, nil) for RFC 8628 soft errors (authorization_pending, slow_down, etc.).
// Returns ("", "", err) for network or parse errors.
func doTokenRequest(client *http.Client, serverURL, deviceCode string) (token string, errCode string, err error) {
	body, _ := json.Marshal(map[string]string{
		"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
		"device_code": deviceCode,
		"client_id":   clientID,
	})
	resp, err := client.Post(
		serverURL+"/api/auth/device/token",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		var tr tokenResponse
		if err := json.Unmarshal(data, &tr); err != nil {
			return "", "", fmt.Errorf("decoding token response: %w", err)
		}
		return tr.AccessToken, "", nil
	}

	var errResp tokenErrorResponse
	if err := json.Unmarshal(data, &errResp); err != nil {
		return "", "", fmt.Errorf("unexpected response %d", resp.StatusCode)
	}
	return "", errResp.Error, nil
}

func pollForToken(client *http.Client, serverURL, deviceCode string, intervalSecs int) (string, error) {
	spinChars := []string{"|", "/", "-", "\\"}
	spinIdx := 0

	for {
		time.Sleep(time.Duration(intervalSecs) * time.Second)
		fmt.Printf("\rWaiting for authorization... %s ", spinChars[spinIdx%len(spinChars)])
		spinIdx++

		token, errCode, err := doTokenRequest(client, serverURL, deviceCode)
		if err != nil {
			fmt.Println()
			return "", fmt.Errorf("polling token endpoint: %w", err)
		}

		if token != "" {
			fmt.Println()
			return token, nil
		}

		switch errCode {
		case "authorization_pending":
			// continue polling
		case "slow_down":
			intervalSecs += 5
		case "expired_token":
			fmt.Println()
			return "", fmt.Errorf("authorization timed out: please run 'spade login' again")
		case "access_denied":
			fmt.Println()
			return "", fmt.Errorf("access denied")
		default:
			fmt.Println()
			return "", fmt.Errorf("authorization error: %s", errCode)
		}
	}
}
