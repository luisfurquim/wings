package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
)

// Upstream coordinates. Pinned to v3.3 (latest stable) for reproducibility.
// Bumping the tag is intentional and should be paired with a smoke-test, so
// the constant lives here rather than behind a flag.
const (
	unitexCoreRepo  = "https://github.com/UnitexGramLab/unitex-core.git"
	unitexCoreTag   = "v3.3"
	unitexLinguaRaw = "https://raw.githubusercontent.com/UnitexGramLab/unitex-lingua/master"
)

// cloneUnitexCore performs a shallow checkout of unitex-core at the pinned tag
// into dst. The clone is reused on subsequent invocations: if dst already
// looks like a populated repository the call is a no-op. Returning early on
// "already present" lets the build step decide whether the binary is current.
func cloneUnitexCore(dst string) error {
	if info, err := os.Stat(filepath.Join(dst, ".git")); err == nil && info.IsDir() {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("mkdir parent: %w", err)
	}
	cmd := exec.Command("git", "clone",
		"--depth", "1",
		"--branch", unitexCoreTag,
		unitexCoreRepo, dst,
	)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone unitex-core: %w", err)
	}
	return nil
}

// fetchDelaSources downloads <Base>.bin and <Base>.inf for the requested
// language into cacheDir. Both files are required for UnitexToolLogger
// Uncompress to reconstruct the text DELAF, so the function fetches them as a
// pair and either succeeds for both or reports the first failure. Existing
// files are left untouched (the unitex-lingua repo is the source of truth, and
// re-downloading would just churn bytes).
func fetchDelaSources(src langSource, cacheDir string) (binPath, infPath string, err error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", "", fmt.Errorf("mkdir cache: %w", err)
	}
	binPath = filepath.Join(cacheDir, src.Base+".bin")
	infPath = filepath.Join(cacheDir, src.Base+".inf")
	for _, item := range []struct {
		name, dst string
	}{
		{src.Base + ".bin", binPath},
		{src.Base + ".inf", infPath},
	} {
		if _, statErr := os.Stat(item.dst); statErr == nil {
			continue
		}
		// PathEscape protects spaces (e.g. "Georgian (Ancient)_may2009.bin")
		// and other reserved characters in upstream filenames.
		raw := unitexLinguaRaw + "/" + src.Subdir + "/Dela/" + url.PathEscape(item.name)
		if err := downloadFile(raw, item.dst); err != nil {
			return "", "", err
		}
	}
	return binPath, infPath, nil
}

// fetchProviderAvatars downloads the avatar PNG for every provider listed in
// providerAvatarURLs into avatarsDir/<provider>.png. Each file is downloaded
// once and cached; subsequent calls are idempotent. Errors are logged to
// stderr but do not abort the build — a missing avatar just means wlate shows
// no badge, which is cosmetic.
func fetchProviderAvatars(avatarsDir string) {
	if err := os.MkdirAll(avatarsDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "warn: cannot create avatars dir %s: %v\n", avatarsDir, err)
		return
	}
	for provider, url := range providerAvatarURLs {
		dst := filepath.Join(avatarsDir, provider+".png")
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		if err := downloadFile(url, dst); err != nil {
			fmt.Fprintf(os.Stderr, "warn: avatar fetch for %q: %v\n", provider, err)
		}
	}
}

// downloadFile streams an HTTP GET into dst, writing through a temporary file
// so an interrupted download cannot leave a half-written artefact in the
// cache. Errors include the URL because dictbuild may be invoked
// non-interactively (CI) and the upstream filename is the most useful clue
// when something 404s.
func downloadFile(url, dst string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("download %s: %w", url, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename temp: %w", err)
	}
	fmt.Fprintf(os.Stderr, "fetched %s\n", filepath.Base(dst))
	return nil
}
