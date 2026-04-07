package sites

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	"github.com/Girbons/comics-downloader/pkg/config"
	"github.com/Girbons/comics-downloader/pkg/core"
	"github.com/Girbons/comics-downloader/pkg/util"
	"github.com/anaskhan96/soup"
)

var baseUrl = "https://readcomiconline.li"

// ReadComicOnline represents a readcomiconline instance.
type ReadComicOnline struct {
	options *config.Options
}

// NewReadComiconline returns a readcomiconline instance.
func NewReadComiconline(options *config.Options) *ReadComicOnline {
	return &ReadComicOnline{
		options: options,
	}
}

func deobfuscateUrl(imageLink string) (string, error) {
	imageLink = strings.ReplaceAll(imageLink, "_x236", "d")
	imageLink = strings.ReplaceAll(imageLink, "_x945", "g")

	if strings.HasPrefix(imageLink, "https://2.bp.blogspot.com") {
		return imageLink, nil
	}

	var quality string

	if idx := strings.Index(imageLink, "=s0?"); idx >= 0 {
		imageLink = imageLink[:idx]
		quality = "=s0"
	} else if idx := strings.Index(imageLink, "=s1600?"); idx >= 0 {
		imageLink = imageLink[:idx]
		quality = "=s1600"
	} else {
		return "", fmt.Errorf("readcomiconline: no quality marker in obfuscated URL")
	}

	// String surgery to reconstruct the base64 payload.
	if len(imageLink) < 25 {
		return "", fmt.Errorf("readcomiconline: obfuscated segment too short (%d chars)", len(imageLink))
	}
	imageLink = imageLink[4:22] + imageLink[25:]
	if len(imageLink) < 6 {
		return "", fmt.Errorf("readcomiconline: segment too short after first surgery")
	}
	imageLink = imageLink[0:len(imageLink)-6] + imageLink[len(imageLink)-2:]

	sd, err := base64.StdEncoding.DecodeString(imageLink)
	if err != nil {
		return "", fmt.Errorf("readcomiconline: base64 decode failed: %w", err)
	}

	decoded := string(sd)
	if len(decoded) < 17 {
		return "", fmt.Errorf("readcomiconline: decoded segment too short (%d chars)", len(decoded))
	}
	decoded = decoded[0:13] + decoded[17:]
	if len(decoded) < 2 {
		return "", fmt.Errorf("readcomiconline: decoded segment too short after second surgery")
	}
	decoded = decoded[0:len(decoded)-2] + quality

	return "https://2.bp.blogspot.com/" + decoded, nil
}

func (c *ReadComicOnline) retrieveImageLinks(comic *core.Comic) ([]string, error) {
	var links []string
	const debugSnippetLimit = 4096

	comic.URLSource = strings.Split(comic.URLSource, "?")[0]
	fetchURL := comic.URLSource + "?quality=hd&readType=1"

	if c.options.Debug && c.options.Logger != nil {
		c.options.Logger.Debugf("readcomiconline: fetching %s", fetchURL)
	}

	response, err := soup.Get(fetchURL)
	if err != nil {
		if c.options.Logger != nil {
			c.options.Logger.Errorf("readcomiconline: request to %s failed: %v", fetchURL, err)
		}
		return nil, fmt.Errorf("readcomiconline: fetch %s: %w", fetchURL, err)
	}

	re := regexp.MustCompile(`push\(\'(.*?)\'\)`)
	match := re.FindAllStringSubmatch(response, -1)

	for i := range match {
		url := match[i][1]

		clearUrl, decErr := deobfuscateUrl(url)
		if decErr != nil {
			if c.options.Debug && c.options.Logger != nil {
				c.options.Logger.Debugf("readcomiconline: skipping undecodable entry: %v", decErr)
			}
			continue
		}

		if util.IsURLValid(clearUrl) && !util.IsValueInSlice(clearUrl, links) {
			links = append(links, clearUrl)
		}
	}

	if len(links) == 0 && len(match) > 0 {
		err = fmt.Errorf("readcomiconline: found %d push() entries but none could be decoded; the site may have updated its obfuscation scheme", len(match))
	}

	if c.options.Debug && c.options.Logger != nil {
		c.options.Logger.Debugf("readcomiconline: found %d obfuscated entries, %d valid links for %s", len(match), len(links), comic.URLSource)
		snippet := response
		if len(snippet) > debugSnippetLimit {
			snippet = snippet[:debugSnippetLimit]
		}
		encoded := base64.StdEncoding.EncodeToString([]byte(snippet))
		c.options.Logger.Debugf("readcomiconline: response snippet (base64, trimmed to %d bytes) = %s", len(snippet), encoded)
		if len(match) > 0 {
			c.options.Logger.Debugf("readcomiconline: first obfuscated entry (base64) = %s", base64.StdEncoding.EncodeToString([]byte(match[0][1])))
		}
		if len(links) > 0 {
			preview := links[0]
			if len(preview) > 256 {
				preview = preview[:256] + "..."
			}
			c.options.Logger.Debugf("readcomiconline: first decoded link = %s", preview)
		}
		c.options.Logger.Debug(fmt.Sprintf("Image Links found: %s", strings.Join(links, " ")))
	}

	return links, err
}

func (c *ReadComicOnline) isSingleIssue(url string) bool {
	parts := util.TrimAndSplitURL(url)
	return len(parts) > 5 && strings.Contains(parts[5], "Issue-")
}

func (c *ReadComicOnline) retrieveLastIssue(url string) (string, error) {
	var lastIssue string

	response, err := soup.Get(url)
	if err != nil {
		return "", err
	}

	name := util.TrimAndSplitURL(url)[4]
	re := regexp.MustCompile("<a[^>]+href=\"([^\">]+" + "/" + name + "/.+)\"")
	match := re.FindAllStringSubmatch(response, -1)
	lastIssue = baseUrl + strings.Split(match[0][1], "?")[0]

	return lastIssue, nil
}

// RetrieveIssueLinks gets a slice of urls for all issues in a comic
func (c *ReadComicOnline) RetrieveIssueLinks() ([]string, error) {
	url := c.options.URL

	if c.options.Last {
		issue, err := c.retrieveLastIssue(url)
		return []string{issue}, err
	}

	if c.options.All && c.isSingleIssue(url) {
		url = baseUrl + "/Comic/" + util.TrimAndSplitURL(url)[3]
	} else if c.isSingleIssue(url) {
		return []string{url}, nil
	}

	name := util.TrimAndSplitURL(url)[4]
	var (
		pages []string
		links []string
	)

	response, err := soup.Get(url)
	if err != nil {
		return nil, err
	}

	pages = append(pages, url)
	re := regexp.MustCompile("<a[^>]+href=\"([^\">]+" + "/" + name + "/.+)\"")
	match := re.FindAllStringSubmatch(response, -1)

	for i := range match {
		url := match[i][1]
		if !util.IsValueInSlice(url, pages) {
			url = baseUrl + strings.Split(url, "?")[0]
			if util.IsURLValid(url) && !util.IsValueInSlice(url, links) {
				links = append(links, url)
			}
		}
	}

	if c.options.Debug {
		c.options.Logger.Debug(fmt.Sprintf("Issues Links retrieved: %s", strings.Join(links, " ")))
	}

	return links, err
}

// GetInfo extracts the basic info from the given url.
func (c *ReadComicOnline) GetInfo(url string) (string, string) {
	parts := util.TrimAndSplitURL(url)
	name := parts[4]
	issueNumber := strings.Split(strings.ReplaceAll(parts[5], "Issue-", ""), "?")[0]

	return name, issueNumber
}

// Initialize will initialize the comic based
// on ReadComicOnline.to
func (c *ReadComicOnline) Initialize(comic *core.Comic) error {
	links, err := c.retrieveImageLinks(comic)
	comic.Links = links

	return err
}
