package pa2skills

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const defaultReleaseBaseURL = "https://github.com/pa2x2/agents/releases"

func UpdateBinary(currentVersion string, checkOnly bool) (string, error) {
	if currentVersion == "development" || currentVersion == "" {
		return "development build; automatic upgrade skipped", nil
	}
	baseURL := strings.TrimRight(os.Getenv("PA2_SKILLS_RELEASE_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = defaultReleaseBaseURL
	}
	client := &http.Client{Timeout: 30 * time.Second}
	request, err := http.NewRequest(http.MethodGet, baseURL+"/latest", nil)
	if err != nil {
		return "", err
	}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("check latest release: %w", err)
	}
	response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 400 {
		return "", fmt.Errorf("check latest release: HTTP %s", response.Status)
	}
	latest := filepath.Base(strings.TrimRight(response.Request.URL.Path, "/"))
	if latest == "latest" || latest == "." || latest == "/" {
		return "", errors.New("latest release did not resolve to a version tag")
	}
	if latest == currentVersion {
		return fmt.Sprintf("already current at %s", currentVersion), nil
	}
	if comparison, comparable := compareReleaseVersions(latest, currentVersion); comparable && comparison <= 0 {
		return fmt.Sprintf("already current at %s (latest release is %s)", currentVersion, latest), nil
	}
	if checkOnly {
		return fmt.Sprintf("upgrade available: %s -> %s", currentVersion, latest), nil
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return "", fmt.Errorf("automatic upgrade is unsupported on %s", runtime.GOOS)
	}
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		return "", fmt.Errorf("automatic upgrade is unsupported on %s", runtime.GOARCH)
	}
	assetName := fmt.Sprintf("pa2-skills_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	asset, err := download(client, fmt.Sprintf("%s/download/%s/%s", baseURL, latest, assetName))
	if err != nil {
		return "", err
	}
	checksums, err := download(client, fmt.Sprintf("%s/download/%s/SHA256SUMS", baseURL, latest))
	if err != nil {
		return "", err
	}
	expected := checksumFor(checksums, assetName)
	actualSum := sha256.Sum256(asset)
	actual := hex.EncodeToString(actualSum[:])
	if expected == "" || expected != actual {
		return "", errors.New("release checksum verification failed")
	}
	binary, err := binaryFromArchive(asset)
	if err != nil {
		return "", err
	}
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate current executable: %w", err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
		executable = resolved
	}
	temporary, err := os.CreateTemp(filepath.Dir(executable), ".pa2-skills-upgrade-")
	if err != nil {
		return "", fmt.Errorf("prepare binary upgrade: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if _, err := temporary.Write(binary); err != nil {
		temporary.Close()
		return "", err
	}
	if err := temporary.Chmod(0o755); err != nil {
		temporary.Close()
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(temporaryName, executable); err != nil {
		return "", fmt.Errorf("activate binary upgrade: %w", err)
	}
	return fmt.Sprintf("updated %s -> %s", currentVersion, latest), nil
}

func download(client *http.Client, url string) ([]byte, error) {
	response, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", filepath.Base(url), err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: HTTP %s", filepath.Base(url), response.Status)
	}
	contents, err := readLimited(response.Body, 128<<20)
	if err != nil {
		return nil, err
	}
	return contents, nil
}

func checksumFor(contents []byte, asset string) string {
	for _, line := range strings.Split(string(contents), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == asset {
			return fields[0]
		}
	}
	return ""
}

func binaryFromArchive(contents []byte) ([]byte, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(contents))
	if err != nil {
		return nil, fmt.Errorf("open release archive: %w", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read release archive: %w", err)
		}
		if header.Typeflag == tar.TypeReg && filepath.Base(header.Name) == "pa2-skills" {
			return readLimited(tarReader, 128<<20)
		}
	}
	return nil, errors.New("release archive does not contain pa2-skills")
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	contents, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(contents)) > limit {
		return nil, errors.New("release file exceeds the size limit")
	}
	return contents, nil
}

func compareReleaseVersions(left, right string) (int, bool) {
	parse := func(value string) ([3]int, bool) {
		var result [3]int
		value = strings.TrimPrefix(value, "v")
		value, _, _ = strings.Cut(value, "-")
		parts := strings.Split(value, ".")
		if len(parts) != len(result) {
			return result, false
		}
		for index, part := range parts {
			number, err := strconv.Atoi(part)
			if err != nil {
				return result, false
			}
			result[index] = number
		}
		return result, true
	}
	leftParts, leftOK := parse(left)
	rightParts, rightOK := parse(right)
	if !leftOK || !rightOK {
		return 0, false
	}
	for index := range leftParts {
		if leftParts[index] < rightParts[index] {
			return -1, true
		}
		if leftParts[index] > rightParts[index] {
			return 1, true
		}
	}
	return 0, true
}
