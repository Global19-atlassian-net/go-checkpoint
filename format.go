package checkpoint

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// homeDir returns the current users home directory irrespecitve of the OS
func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

// configDir returns the config directory for solo.io
func configDir() (string, error) {
	d := filepath.Join(homeDir(), ".soloio")
	_, err := os.Stat(d)
	if err == nil {
		return d, nil
	}
	if os.IsNotExist(err) {
		err = os.Mkdir(d, 0755)
		if err != nil {
			return "", err
		}
		return d, nil
	}

	return d, err
}

func getSigfile() string {
	sigfile := filepath.Join(homeDir(), ".soloio.sig")
	configDir, err := configDir()
	if err == nil {
		sigfile = filepath.Join(configDir, "soloio.sig")
	}
	return sigfile
}

// callReport calls a basic version check
func callReport(product string, version string, t time.Time) {
	sigfile := getSigfile()
	ctx := context.Background()
	reportParams := &ReportParams{
		Product:       product,
		Version:       version,
		StartTime:     t,
		EndTime:       time.Now(),
		SignatureFile: sigfile,
		Type:          "r1",
	}
	report(ctx, reportParams)
}

func getCheckInputs(product, version string) (*CheckParams, func(resp *CheckResponse, err error)) {
	signature, err := checkSignature(getSigfile())
	if err != nil {
		signature, err = generateSignature()
		if err != nil {
			signature = "siggenerror"
		}
	}
	params := &CheckParams{
		Product:   product,
		Version:   version,
		Signature: signature,
		Type:      "c1",
	}
	cb := func(resp *CheckResponse, err error) {
		if err != nil {
			return
		}
		if resp.Outdated && resp.CurrentVersion != "" && resp.CurrentVersion != version {
			fmt.Printf("A new version of %v is available. Please visit %v.\n", product, resp.CurrentDownloadURL)
		}
		return
	}
	return params, cb
}

// callCheck calls a basic version check at an interval
func callCheck(product string, version string, t time.Time) {
	params, cb := getCheckInputs(product, version)
	checkInterval(params, VersionCheckInterval, cb)
}

// callCheck calls a basic version check at an interval
func callCheckOnceNow(product string, version string) {
	params, cb := getCheckInputs(product, version)
	resp, err := check(params)
	cb(resp, err)
}
