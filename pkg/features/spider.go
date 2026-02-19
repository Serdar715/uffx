package features

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/sw33tLie/uff/v2/pkg/ffuf"
)

type SpiderHook struct {
	// We might add scope options here later
}

func NewSpiderHook() *SpiderHook {
	return &SpiderHook{}
}

func (h *SpiderHook) Name() string {
	return "Spider / Crawler"
}

func (h *SpiderHook) Execute(resp *ffuf.Response, req *ffuf.Request) error {
	// Only spider text/html content
	if !strings.Contains(resp.ContentType, "text") && !strings.Contains(resp.ContentType, "xml") && !strings.Contains(resp.ContentType, "json") {
		return nil
	}

	// If it's JSON/XML, goquery might not be best, but often APIs return HTML error pages or mixed content.
	// For pure JSON, we might miss links unless we regex, but DOM parser is safer for HTML.
	// Let's stick to DOM for HTML and fallback/skip for others?
	// Actually, strict HTML parsing is safer.

	if !strings.Contains(resp.ContentType, "html") {
		// If strictly not HTML, maybe skip or use simple regex as fallback?
		// For now, let's just support HTML crawling as that's the main use case for Spider.
		return nil
	}

	baseURL, err := url.Parse(req.Url)
	if err != nil {
		return nil
	}

	foundLinks := make(map[string]bool)

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(resp.Data))
	if err != nil {
		// Fallback or ignore
		return nil
	}

	// Helper to handle link
	handleLink := func(val string) {
		val = strings.TrimSpace(val)
		if val == "" || strings.HasPrefix(val, "#") || strings.HasPrefix(val, "javascript:") || strings.HasPrefix(val, "mailto:") {
			return
		}

		u, err := url.Parse(val)
		if err != nil {
			return
		}
		resolvedURL := baseURL.ResolveReference(u).String()

		// Scope check: Start with same host
		if !strings.HasPrefix(resolvedURL, baseURL.Scheme+"://"+baseURL.Host) {
			return
		}

		// Remove frag
		if idx := strings.Index(resolvedURL, "#"); idx != -1 {
			resolvedURL = resolvedURL[:idx]
		}

		foundLinks[resolvedURL] = true
	}

	// Select common link elements
	// a[href], area[href], link[href]
	doc.Find("a[href], area[href], link[href]").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			handleLink(href)
		}
	})

	// script[src], img[src], iframe[src], embed[src], source[src], track[src]
	doc.Find("script[src], img[src], iframe[src], embed[src], source[src], track[src]").Each(func(i int, s *goquery.Selection) {
		if src, exists := s.Attr("src"); exists {
			handleLink(src)
		}
	})

	// form[action]
	doc.Find("form[action]").Each(func(i int, s *goquery.Selection) {
		if action, exists := s.Attr("action"); exists {
			handleLink(action)
		}
	})

	for link := range foundLinks {
		resp.NewTargets = append(resp.NewTargets, link)
	}

	// Output info via ScraperData for visibility without breaking JSON
	if len(foundLinks) > 0 {
		if resp.ScraperData == nil {
			resp.ScraperData = make(map[string][]string)
		}
		resp.ScraperData["spider_links_found"] = []string{fmt.Sprintf("%d", len(foundLinks))}
	}

	return nil
}

var _ ffuf.PostResponseHook = &SpiderHook{}
