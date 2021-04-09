package main

import (
	_ "embed"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"

	"github.com/urfave/cli/v2"
)

const baseUrl = "https://raw.githubusercontent.com/ethereum/eth2.0-specs/dev"

// Regex to find Python's code snippets in markdown.
var reg2 = regexp.MustCompile(`(?msU)^\x60\x60\x60python(.*)^\x60\x60\x60`)

func download(cliCtx *cli.Context) error {
	baseDir := cliCtx.String(dirFlag.Name)
	for dirName, fileNames := range specDirs {
		if err := prepareDir(path.Join(baseDir, dirName)); err != nil {
			return err
		}
		for _, fileName := range fileNames {
			outFilePath := path.Join(baseDir, dirName, fileName)
			specDocUrl := fmt.Sprintf("%s/%s", baseUrl, fmt.Sprintf("%s/%s", dirName, fileName))
			if err := getAndSaveFile(specDocUrl, outFilePath); err != nil {
				return err
			}
		}
	}

	return nil
}

func getAndSaveFile(specDocUrl, outFilePath string) error {
	// Create output file.
	f, err := os.Create(outFilePath)
	if err != nil {
		return fmt.Errorf("cannot create output file: %w", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Printf("cannot close output file: %v", err)
		}
	}()

	// Download spec doc.
	fmt.Printf("URL: %v\n", specDocUrl)
	resp, err := http.Get(specDocUrl)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("cannot close spec doc file: %v", err)
		}
	}()

	// Transform and save spec docs.
	specDoc, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	specDocString := string(specDoc)
	for _, snippet := range reg2.FindAllString(specDocString, -1) {
		fmt.Printf("Snippet:\n>>%v<<\n\n", snippet)
		if _, err = f.WriteString(snippet + "\n"); err != nil {
			return err
		}
	}

	fmt.Printf("f: %v, path: %v\n", f, outFilePath)

	return nil
}

func prepareDir(dirPath string) error {
	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		return err
	}
	return nil
}
