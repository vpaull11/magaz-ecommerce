package handler

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"time"

	"magaz/internal/repository"
	"magaz/internal/service"
)

// SEOHandler serves /sitemap.xml and /robots.txt
type SEOHandler struct {
	*Base
	productRepo  *repository.ProductRepository
	categoryRepo *repository.CategoryRepository
	settingsSvc  *service.SettingsService
}

func NewSEOHandler(base *Base, productRepo *repository.ProductRepository, categoryRepo *repository.CategoryRepository, settingsSvc *service.SettingsService) *SEOHandler {
	return &SEOHandler{
		Base:         base,
		productRepo:  productRepo,
		categoryRepo: categoryRepo,
		settingsSvc:  settingsSvc,
	}
}

// --- Sitemap ---------------------------------------------------------------

type urlset struct {
	XMLName xml.Name  `xml:"urlset"`
	Xmlns   string    `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	Priority   string `xml:"priority,omitempty"`
}

// GET /sitemap.xml
func (h *SEOHandler) Sitemap(w http.ResponseWriter, r *http.Request) {
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	base := fmt.Sprintf("%s://%s", scheme, r.Host)
	today := time.Now().Format("2006-01-02")

	urls := []sitemapURL{
		{Loc: base + "/", ChangeFreq: "daily", Priority: "1.0", LastMod: today},
		{Loc: base + "/catalog", ChangeFreq: "daily", Priority: "0.9", LastMod: today},
	}

	// All active categories
	cats, _ := h.categoryRepo.List()
	for _, c := range cats {
		urls = append(urls, sitemapURL{
			Loc:        base + "/catalog/" + c.Slug,
			ChangeFreq: "daily",
			Priority:   "0.8",
			LastMod:    today,
		})
	}

	// All active products
	products, _, _ := h.productRepo.List(nil, 10000, 0, "", nil)
	for _, p := range products {
		urls = append(urls, sitemapURL{
			Loc:        fmt.Sprintf("%s/catalog/item/%d", base, p.ID),
			ChangeFreq: "weekly",
			Priority:   "0.7",
		})
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	enc.Encode(urlset{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	})
}

// GET /robots.txt
func (h *SEOHandler) Robots(w http.ResponseWriter, r *http.Request) {
	txt := h.settingsSvc.Get("robots_txt",
		"User-agent: *\nAllow: /\nDisallow: /admin/\nSitemap: /sitemap.xml")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, txt)
}
