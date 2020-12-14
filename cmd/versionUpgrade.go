package cmd

import (
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
)

var downloadURL = "https://github.com/karimra/gnmic/raw/master/install.sh"

// upgradeCmd represents the version command
var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "upgrade gnmic to latest available version",

	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := ioutil.TempFile("", "gnmic")
		defer os.Remove(f.Name())
		if err != nil {
			return err
		}
		err = downloadFile(downloadURL, f)
		if err != nil {
			return err
		}

		c := exec.Command("bash", f.Name())
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		err = c.Run()
		if err != nil {
			return err
		}
		return nil
	},
}

// downloadFile will download a file from a URL and write its content to a file
func downloadFile(url string, file *os.File) error {
	client := http.Client{Timeout: 10 * time.Second}
	// Get the data
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

func init() {
	versionCmd.AddCommand(upgradeCmd)
}
