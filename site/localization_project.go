package site

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/reidransom/jigyll/config"
	"github.com/reidransom/jigyll/localization"
	"github.com/reidransom/jigyll/pages"
	"github.com/reidransom/jigyll/plugins"
	"github.com/reidransom/jigyll/utils"
)

// LocalizedBuild enters the project-level production seam for an opt-in
// localized site. It prepares isolated locale sites, renders them serially to
// one sibling staging tree, and promotes that complete generation only after
// every locale has succeeded.
// Callers that need a production serving or watch snapshot should use
// BuildLocalizedProject. Development servers should use
// BuildLocalizedDevelopmentProject to retain its immutable served documents.
func LocalizedBuild(base *Site) (int, error) {
	_, count, err := BuildLocalizedProject(base)
	return count, err
}

// LocalizedProject is one successfully built localization generation. Its
// locale sites and aggregate route index are immutable after construction, so
// callers can replace one project pointer without exposing a mixed generation.
type LocalizedProject struct {
	project         *localizationProject
	routes          map[string]localizedRoute
	servedDocuments map[string]LocalizedServedDocument
}

// LocalizedServedDocument is one immutable response from a completed
// development generation. Its document and site retain response metadata while
// RenderTo supplies the materialized bytes without reading mutable source files.
type LocalizedServedDocument struct {
	site     *Site
	document Document
	content  string
}

// Site returns the locale site that owns the served document.
func (d LocalizedServedDocument) Site() *Site {
	return d.site
}

// Document returns the document metadata for the served response.
func (d LocalizedServedDocument) Document() Document {
	return d.document
}

// RenderTo writes the immutable materialized response.
func (d LocalizedServedDocument) RenderTo(w io.Writer) error {
	_, err := io.WriteString(w, d.content)
	return err
}

type localizedRoute struct {
	site     *Site
	document Document
}

// BuildLocalizedProject builds and publishes one complete localization
// generation, returning its aggregate serving snapshot only after promotion
// succeeds.
func BuildLocalizedProject(base *Site) (*LocalizedProject, int, error) {
	return buildLocalizedProject(base, productionPublication{})
}

// BuildLocalizedDevelopmentProject constructs one complete localized
// development generation in memory. It does not create, clean, write, or
// promote the configured destination.
func BuildLocalizedDevelopmentProject(base *Site) (*LocalizedProject, int, error) {
	return buildLocalizedProject(base, newDevelopmentPublication())
}

func buildLocalizedProject(base *Site, publication localizedPublication) (*LocalizedProject, int, error) {
	if base == nil || !base.cfg.Enabled() {
		return nil, 0, fmt.Errorf("localized build requires localization configuration")
	}
	project, err := newLocalizationProject(base)
	if err != nil {
		return nil, 0, err
	}
	count, servedDocuments, err := project.build(publication)
	if err != nil {
		return nil, count, err
	}
	return &LocalizedProject{
		project:         project,
		routes:          project.routes,
		servedDocuments: servedDocuments,
	}, count, nil
}

// Config returns the shared project configuration.
func (p *LocalizedProject) Config() *config.Config {
	return p.project.base.Config()
}

// WatchFiles observes the project source once. Every emitted event requires a
// complete fresh generation: Rebuild publishes production output, while
// RebuildDevelopment materializes a development snapshot.
func (p *LocalizedProject) WatchFiles() (<-chan FilesEvent, error) {
	return p.project.base.WatchFiles()
}

// Rebuild reads a fresh project and publishes a complete replacement
// generation. The current project remains usable when the rebuild fails.
func (p *LocalizedProject) Rebuild() (*LocalizedProject, int, error) {
	return p.rebuild(BuildLocalizedProject)
}

// RebuildDevelopment reads a fresh project and materializes a complete
// replacement development generation. The current project remains usable when
// the rebuild fails.
func (p *LocalizedProject) RebuildDevelopment() (*LocalizedProject, int, error) {
	return p.rebuild(BuildLocalizedDevelopmentProject)
}

func (p *LocalizedProject) rebuild(build func(*Site) (*LocalizedProject, int, error)) (*LocalizedProject, int, error) {
	fresh, err := FromDirectory(p.project.base.SourceDir(), p.project.base.flags)
	if err != nil {
		return nil, 0, err
	}
	fresh.SetAbsoluteURL(p.project.base.cfg.AbsoluteURL)
	return build(fresh)
}

// SetAbsoluteURL applies the serving URL to every locale before pages are
// served. Rebuild carries this value into the next project generation.
func (p *LocalizedProject) SetAbsoluteURL(url string) {
	p.project.base.SetAbsoluteURL(url)
	for _, localeSite := range p.project.prepared {
		localeSite.SetAbsoluteURL(url)
	}
}

// URLPage resolves a request through the aggregate, validated route index and
// returns the owning locale site needed to render the document.
func (p *LocalizedProject) URLPage(urlpath string) (*Site, Document, bool) {
	route, found := p.routes[urlpath]
	if !found {
		return nil, nil, false
	}
	return route.site, route.document, true
}

// ServedDocument resolves a route to the immutable response materialized by a
// completed development generation. Production projects have no served
// documents because they publish to their destination tree instead.
func (p *LocalizedProject) ServedDocument(urlpath string) (LocalizedServedDocument, bool) {
	document, found := p.servedDocuments[urlpath]
	return document, found
}

type localizationProject struct {
	base               *Site
	catalog            *localization.Catalog
	data               *localization.DataCatalog
	locales            []config.Locale
	prepared           []*Site
	routes             map[string]localizedRoute
	aggregateRoutes    []aggregateRoute
	aggregateDocuments []aggregateDocument
}

// aggregateRoute is the project-level handoff for aggregate generators. An
// aggregate producer must register every route it will write before project
// validation, so aggregate output cannot overwrite locale or shared output.
type aggregateRoute struct {
	Route  string
	Source string
}

func (p *localizationProject) registerAggregateRoute(route, source string) {
	p.aggregateRoutes = append(p.aggregateRoutes, aggregateRoute{Route: route, Source: source})
}

// aggregateDocument is one project-level document and the prepared locale site
// that supplies its output destination and rendering hooks.
type aggregateDocument struct {
	site     *Site
	document Document
}

type localizedSitemapDocument struct {
	pages.PageEmbed
	content string
}

func (d *localizedSitemapDocument) Write(w io.Writer) error {
	_, err := io.WriteString(w, d.content)
	return err
}

func (p *localizationProject) prepareAggregateDocuments() error {
	if !p.sitemapConfigured() {
		return nil
	}

	sitemap := &localizedSitemapDocument{
		PageEmbed: pages.PageEmbed{Path: "/sitemap.xml"},
		content:   plugins.RenderLocalizedSitemap(p.localizedSitemapEntries()),
	}
	p.aggregateDocuments = append(p.aggregateDocuments, aggregateDocument{
		site:     p.prepared[0],
		document: sitemap,
	})
	p.registerAggregateRoute(sitemap.URL(), "jekyll-sitemap")

	if !p.hasRootRoute("/robots.txt") {
		robots := &localizedSitemapDocument{
			PageEmbed: pages.PageEmbed{Path: "/robots.txt"},
			content:   "Sitemap: " + utils.URLJoin(p.base.cfg.AbsoluteURL, p.base.cfg.BaseURL, "/sitemap.xml"),
		}
		p.aggregateDocuments = append(p.aggregateDocuments, aggregateDocument{
			site:     p.prepared[0],
			document: robots,
		})
		p.registerAggregateRoute(robots.URL(), "jekyll-sitemap")
	}
	return nil
}

func (p *localizationProject) sitemapConfigured() bool {
	for _, site := range p.prepared {
		if _, found := site.pluginInstances["jekyll-sitemap"]; found {
			return true
		}
	}
	return false
}

func (p *localizationProject) hasRootRoute(route string) bool {
	for _, site := range p.prepared {
		if site.HasRoute(route) {
			return true
		}
	}
	return false
}

type localizedSitemapEntry struct {
	site     *Site
	page     pages.Page
	entry    plugins.SitemapEntry
	identity localization.Identity
}

func (p *localizationProject) localizedSitemapEntries() []plugins.SitemapEntry {
	entries := p.collectLocalizedSitemapEntries(sitemapEditions(p.catalog))
	assignSitemapAlternates(entries, p.base.cfg.Localization.DefaultLanguage)
	result := make([]plugins.SitemapEntry, len(entries))
	for index, entry := range entries {
		result[index] = entry.entry
	}
	return result
}

func sitemapEditions(catalog *localization.Catalog) map[string]localization.Edition {
	editions := make(map[string]localization.Edition)
	for _, input := range catalog.PreparedInputs() {
		for _, edition := range input.Documents {
			editions[edition.Source] = edition
		}
	}
	return editions
}

func (p *localizationProject) collectLocalizedSitemapEntries(editions map[string]localization.Edition) []localizedSitemapEntry {
	entries := make([]localizedSitemapEntry, 0)
	for _, site := range p.prepared {
		for _, document := range site.OutputDocs() {
			if sitemapEligible(site, document) {
				entries = append(entries, localizedSitemapEntryFor(site, document, editions))
			}
		}
	}
	return entries
}

func localizedSitemapEntryFor(site *Site, document Document, editions map[string]localization.Edition) localizedSitemapEntry {
	entry := localizedSitemapEntry{
		site: site,
		entry: plugins.SitemapEntry{
			URL:          sitemapAbsoluteURL(site, document.URL()),
			LastModified: sitemapLastModified(document),
		},
	}
	page, ok := document.(pages.Page)
	if !ok {
		return entry
	}
	entry.page = page
	if edition, found := editions[page.Source()]; found && edition.TranslationKey != "" {
		entry.identity = localization.Identity{Namespace: edition.Namespace, TranslationKey: edition.TranslationKey}
	}
	return entry
}

func assignSitemapAlternates(entries []localizedSitemapEntry, defaultLanguage string) {
	groups := make(map[localization.Identity][]int)
	for index, entry := range entries {
		if entry.page != nil && entry.identity.TranslationKey != "" {
			groups[entry.identity] = append(groups[entry.identity], index)
		}
	}
	for _, group := range groups {
		alternates := sitemapAlternates(entries, group, defaultLanguage)
		for _, index := range group {
			entries[index].entry.Alternates = append([]plugins.SitemapAlternate(nil), alternates...)
		}
	}
}

func sitemapAlternates(entries []localizedSitemapEntry, group []int, defaultLanguage string) []plugins.SitemapAlternate {
	alternates := make([]plugins.SitemapAlternate, 0, len(group))
	for _, index := range group {
		entry := entries[index]
		alternates = append(alternates, plugins.SitemapAlternate{
			Language: entry.site.localizationContext.locale.Tag,
			URL:      entry.entry.URL,
			XDefault: entry.site.localeKey == defaultLanguage,
		})
	}
	return alternates
}

func sitemapEligible(_ *Site, document Document) bool {
	if document.IsStatic() {
		return false
	}

	page, ok := document.(pages.Page)
	if !ok || !page.FrontMatter().Bool("sitemap", true) || sitemapNoIndex(page) {
		return false
	}
	return page.OutputExt() == ".html" && path.Base(page.URL()) != "404.html"
}

func sitemapNoIndex(page pages.Page) bool {
	for _, value := range page.FrontMatter().StringArray("robots") {
		for _, directive := range strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || unicode.IsSpace(r)
		}) {
			if strings.EqualFold(directive, "noindex") {
				return true
			}
		}
	}
	return false
}

func sitemapAbsoluteURL(site *Site, route string) string {
	return utils.URLJoin(site.cfg.AbsoluteURL, site.cfg.BaseURL, strings.ReplaceAll(route, "/index.html", "/"))
}

func sitemapLastModified(document Document) string {
	page, ok := document.(pages.Page)
	if !ok {
		return ""
	}
	if modified, ok := page.FrontMatter()["last_modified_at"].(time.Time); ok {
		return modified.Format("2006-01-02T15:04:05-07:00")
	}
	if page.IsPost() {
		return page.PostDate().Format("2006-01-02T15:04:05-07:00")
	}
	return ""
}

func newLocalizationProject(base *Site) (*localizationProject, error) {
	if err := base.cfg.Localization.Validate(); err != nil {
		return nil, err
	}
	if base.cfg.Incremental {
		return nil, fmt.Errorf("localized builds do not support incremental mode")
	}
	documents, err := discoverLocalizedDocuments(base)
	if err != nil {
		return nil, err
	}
	catalog, err := localization.BuildCatalog(base.cfg.Localization, documents)
	if err != nil {
		return nil, err
	}
	data, err := localization.DiscoverData(filepath.Join(base.SourceDir(), base.cfg.DataDir), base.cfg.Localization)
	if err != nil {
		return nil, err
	}
	return &localizationProject{
		base:    base,
		catalog: catalog,
		data:    data,
		locales: base.cfg.Localization.OrderedLocales(),
	}, nil
}

// build constructs locale sites in deterministic order before rendering and
// publishing each output through the selected materialization policy.
func (p *localizationProject) build(publication localizedPublication) (int, map[string]LocalizedServedDocument, error) {
	stage, cleanup, err := publication.start(p)
	if err != nil {
		return 0, nil, err
	}
	defer cleanup()
	if err := p.prepareLocalizedGeneration(stage); err != nil {
		return 0, nil, err
	}
	routes, err := p.routeIndex()
	if err != nil {
		return 0, nil, err
	}
	count, err := p.renderAndMaterialize(publication)
	if err != nil {
		return count, nil, err
	}
	if err := publication.finish(p, stage); err != nil {
		return count, nil, err
	}
	p.routes = routes
	return count, publication.servedDocuments(), nil
}

func (p *localizationProject) localizedBuildStage() (string, func(), error) {
	if p.base.cfg.DryRun {
		return "", func() {}, nil
	}
	stage, err := os.MkdirTemp(filepath.Dir(p.base.DestDir()), ".jigyll-localized-")
	if err != nil {
		return "", nil, err
	}
	if err := copyKeepFiles(p.base.DestDir(), stage, p.base.cfg.KeepFiles); err != nil {
		os.RemoveAll(stage)
		return "", nil, err
	}
	return stage, func() { os.RemoveAll(stage) }, nil
}

func (p *localizationProject) localizedInputs() map[string]map[string]struct{} {
	inputs := make(map[string]map[string]struct{}, len(p.locales))
	for _, input := range p.catalog.PreparedInputs() {
		sources := make(map[string]struct{}, len(input.Documents))
		for _, edition := range input.Documents {
			sources[edition.Source] = struct{}{}
		}
		inputs[input.Locale.Key] = sources
	}
	return inputs
}

func (p *localizationProject) prepareLocaleSites(stage string, inputs map[string]map[string]struct{}) error {
	for index, locale := range p.locales {
		site, err := p.prepareLocaleSite(stage, inputs[locale.Key], locale, index == 0)
		if err != nil {
			return err
		}
		p.prepared = append(p.prepared, site)
	}
	return nil
}

func (p *localizationProject) prepareLocalizedGeneration(stage string) error {
	if err := p.prepareLocaleSites(stage, p.localizedInputs()); err != nil {
		return err
	}
	if err := p.prepareAggregateDocuments(); err != nil {
		return err
	}
	if err := p.validateRoutes(); err != nil {
		return err
	}
	return p.bindLocalizationContexts()
}

func (p *localizationProject) prepareLocaleSite(stage string, sources map[string]struct{}, locale config.Locale, includeStatic bool) (*Site, error) {
	localeData, err := p.data.Data(locale.Key)
	if err != nil {
		return nil, err
	}
	derived, err := p.base.cfg.DeriveLocale(locale.Key)
	if err != nil {
		return nil, err
	}
	messages, err := p.data.Messages(locale.Key)
	if err != nil {
		return nil, err
	}
	if stage != "" {
		derived.Destination = stage
	}
	site := New(p.base.flags)
	site.cfg = derived
	site.localeKey = locale.Key
	site.localePrefix = localeRoutePrefix(p.base.cfg.Localization, locale)
	site.includeStatic = includeStatic
	site.data = localeData
	site.localizedSources = sources
	site.localizationContext = &localizedSiteContext{
		site:         site,
		locale:       locale,
		registry:     p.base.cfg.Localization,
		messages:     messages,
		pageInfo:     make(map[pages.Page]localizedPageInfo),
		routePages:   make(map[string]pages.Page),
		sharedAssets: make(map[string]struct{}),
	}
	if err := site.Read(); err != nil {
		return nil, fmt.Errorf("preparing locale %q: %w", locale.Key, err)
	}
	return site, nil
}

func (p *localizationProject) renderAndMaterialize(publication localizedPublication) (int, error) {
	count := 0
	for _, localeSite := range p.prepared {
		if err := localeSite.setTimeZone(); err != nil {
			return count, err
		}
		if err := localeSite.ensureRendered(); err != nil {
			return count, fmt.Errorf("rendering locale %q: %w", localeSite.localeKey, err)
		}
		for _, document := range localeSite.OutputDocs() {
			if !publication.includeLocaleDocument(localeSite, document) {
				continue
			}
			count++
			if err := publication.materializeLocaleDocument(localeSite, document); err != nil {
				return count, err
			}
		}
	}
	for _, aggregate := range p.aggregateDocuments {
		count++
		if err := publication.materializeAggregateDocument(aggregate); err != nil {
			return count, err
		}
	}
	return count, nil
}

type localizedPublication interface {
	start(*localizationProject) (string, func(), error)
	includeLocaleDocument(*Site, Document) bool
	materializeLocaleDocument(*Site, Document) error
	materializeAggregateDocument(aggregateDocument) error
	finish(*localizationProject, string) error
	servedDocuments() map[string]LocalizedServedDocument
}

type productionPublication struct{}

func (productionPublication) start(p *localizationProject) (string, func(), error) {
	return p.localizedBuildStage()
}

func (productionPublication) includeLocaleDocument(*Site, Document) bool {
	return true
}

func (productionPublication) materializeLocaleDocument(localeSite *Site, document Document) error {
	if localeSite.cfg.DryRun {
		return nil
	}
	if err := localeSite.WriteDoc(document); err != nil {
		return fmt.Errorf("writing locale %q: %w", localeSite.localeKey, err)
	}
	return nil
}

func (productionPublication) materializeAggregateDocument(aggregate aggregateDocument) error {
	return writeAggregateDocument(aggregate, aggregate.site.cfg.DryRun)
}

func (productionPublication) finish(p *localizationProject, stage string) error {
	if p.base.cfg.DryRun {
		return nil
	}
	return promoteLocalizedGeneration(stage, p.base.DestDir())
}

func (productionPublication) servedDocuments() map[string]LocalizedServedDocument {
	return nil
}

type developmentPublication struct {
	documents map[string]LocalizedServedDocument
}

func newDevelopmentPublication() *developmentPublication {
	return &developmentPublication{documents: make(map[string]LocalizedServedDocument)}
}

func (*developmentPublication) start(*localizationProject) (string, func(), error) {
	return "", func() {}, nil
}

func (*developmentPublication) includeLocaleDocument(localeSite *Site, document Document) bool {
	_, removed := localeSite.removedRoutes[document.URL()]
	return !removed
}

func (publication *developmentPublication) materializeLocaleDocument(localeSite *Site, document Document) error {
	served, err := materializeLocalizedDocument(localeSite, document)
	if err != nil {
		return fmt.Errorf("materializing locale %q document %q: %w", localeSite.localeKey, document.URL(), err)
	}
	publication.record(document, served)
	return nil
}

func (publication *developmentPublication) materializeAggregateDocument(aggregate aggregateDocument) error {
	served, err := materializeLocalizedDocument(aggregate.site, aggregate.document)
	if err != nil {
		return fmt.Errorf("materializing aggregate %q: %w", aggregate.document.URL(), err)
	}
	publication.record(aggregate.document, served)
	return nil
}

func (*developmentPublication) finish(*localizationProject, string) error {
	return nil
}

func (publication *developmentPublication) servedDocuments() map[string]LocalizedServedDocument {
	return publication.documents
}

func (publication *developmentPublication) record(document Document, served LocalizedServedDocument) {
	for _, alias := range localizedRouteAliases(document.URL()) {
		publication.documents[alias] = served
	}
}

func materializeLocalizedDocument(site *Site, document Document) (LocalizedServedDocument, error) {
	var content bytes.Buffer
	if err := site.WriteDocument(&content, document); err != nil {
		return LocalizedServedDocument{}, err
	}
	return LocalizedServedDocument{
		site:     site,
		document: document,
		content:  content.String(),
	}, nil
}

func writeAggregateDocument(aggregate aggregateDocument, dryRun bool) error {
	if dryRun {
		if err := aggregate.document.Write(io.Discard); err != nil {
			return fmt.Errorf("rendering aggregate %q: %w", aggregate.document.URL(), err)
		}
		return nil
	}
	if err := aggregate.site.WriteDoc(aggregate.document); err != nil {
		return fmt.Errorf("writing aggregate %q: %w", aggregate.document.URL(), err)
	}
	return nil
}

// routeIndex records every validated route and its owning site before
// publication. It is installed only after the complete generation succeeds.
func (p *localizationProject) routeIndex() (map[string]localizedRoute, error) {
	routes := make(map[string]localizedRoute)
	for _, localeSite := range p.prepared {
		for _, document := range localeSite.OutputDocs() {
			if _, removed := localeSite.removedRoutes[document.URL()]; removed {
				continue
			}
			route := localizedRoute{site: localeSite, document: document}
			for _, alias := range localizedRouteAliases(document.URL()) {
				if existing, found := routes[alias]; found && existing.document != document {
					return nil, fmt.Errorf("localized route index collision at %q", alias)
				}
				routes[alias] = route
			}
		}
	}
	for _, aggregate := range p.aggregateDocuments {
		route := localizedRoute(aggregate)
		for _, alias := range localizedRouteAliases(aggregate.document.URL()) {
			if existing, found := routes[alias]; found && existing.document != aggregate.document {
				return nil, fmt.Errorf("localized route index collision at %q", alias)
			}
			routes[alias] = route
		}
	}
	return routes, nil
}

func (p *localizationProject) bindLocalizationContexts() error {
	editions := sitemapEditions(p.catalog)
	pagesBySite, groups := localizedPageGroups(p.prepared, editions)
	sharedAssets := localizedSharedAssets(p.prepared)
	for _, site := range p.prepared {
		if err := p.bindLocalizationContext(site, pagesBySite, groups, editions); err != nil {
			return err
		}
		site.localizationContext.sharedAssets = sharedAssets
	}
	return nil
}

func localizedPageGroups(sites []*Site, editions map[string]localization.Edition) (map[*Site][]pages.Page, map[localization.Identity]map[string]pages.Page) {
	pagesBySite := make(map[*Site][]pages.Page, len(sites))
	groups := make(map[localization.Identity]map[string]pages.Page)
	for _, site := range sites {
		pagesBySite[site] = site.Pages()
		for _, page := range pagesBySite[site] {
			edition, found := editions[page.Source()]
			if found && edition.TranslationKey != "" {
				identity := localization.Identity{Namespace: edition.Namespace, TranslationKey: edition.TranslationKey}
				if groups[identity] == nil {
					groups[identity] = make(map[string]pages.Page)
				}
				groups[identity][edition.Locale.Key] = page
			}
		}
	}
	return pagesBySite, groups
}

func localizedSharedAssets(sites []*Site) map[string]struct{} {
	assets := make(map[string]struct{})
	for _, site := range sites {
		for _, document := range site.OutputDocs() {
			if document.IsStatic() {
				assets[document.URL()] = struct{}{}
			}
		}
	}
	return assets
}

func (p *localizationProject) bindLocalizationContext(site *Site, pagesBySite map[*Site][]pages.Page, groups map[localization.Identity]map[string]pages.Page, editions map[string]localization.Edition) error {
	context := site.localizationContext
	for _, candidateSite := range p.prepared {
		locale, found := p.base.cfg.Localization.Locale(candidateSite.localeKey)
		if !found {
			return fmt.Errorf("prepared locale %q is not configured", candidateSite.localeKey)
		}
		for _, page := range pagesBySite[candidateSite] {
			context.bindLocalizedPage(site, candidateSite, page, locale, localizedPageIdentity(page, editions), groups)
		}
	}
	return nil
}

func localizedPageIdentity(page pages.Page, editions map[string]localization.Edition) localization.Identity {
	edition := editions[page.Source()]
	return localization.Identity{Namespace: edition.Namespace, TranslationKey: edition.TranslationKey}
}

func (c *localizedSiteContext) bindLocalizedPage(site, candidateSite *Site, page pages.Page, locale config.Locale, identity localization.Identity, groups map[localization.Identity]map[string]pages.Page) {
	var all map[string]pages.Page
	if identity.TranslationKey != "" {
		all = groups[identity]
	}
	c.pageInfo[page] = localizedPageInfo{identity: identity, locale: locale, page: page, all: all}
	c.routePages[page.URL()] = page
	if candidateSite == site {
		c.routePages[localeRelativeRoute(site.localePrefix, page.URL())] = page
	}
}

func localeRoutePrefix(localizationConfig *config.LocalizationConfig, locale config.Locale) string {
	if locale.Default && !localizationConfig.DefaultLanguageInSubdir {
		return ""
	}
	return locale.Key
}

func discoverLocalizedDocuments(site *Site) ([]localization.Document, error) {
	var documents []localization.Document
	discover := func(source, relativePath, namespace, typename string) error {
		document, err := localization.DiscoverDocument(&site.cfg, source, relativePath, namespace, typename)
		if err != nil {
			return err
		}
		if !localizedDocumentEligible(document, site.cfg) {
			document.Included = false
		}
		documents = append(documents, document)
		return nil
	}
	if err := discoverLocalizedPages(site, discover); err != nil {
		return nil, err
	}
	if err := discoverLocalizedCollections(site, discover); err != nil {
		return nil, err
	}
	return documents, nil
}

func discoverLocalizedPages(site *Site, discover func(string, string, string, string) error) error {
	err := filepath.Walk(site.SourceDir(), func(filename string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relativePath, err := filepath.Rel(site.SourceDir(), filename)
		if err != nil {
			return err
		}
		relativePath = filepath.ToSlash(relativePath)
		if info.IsDir() {
			if relativePath != "." && (site.Exclude(relativePath) || strings.HasPrefix(filepath.Base(relativePath), "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if site.Exclude(relativePath) || strings.HasPrefix(relativePath, "_") {
			return nil
		}
		return discover(filename, relativePath, localization.PagesNamespace, "pages")
	})
	if err != nil {
		return fmt.Errorf("discovering localized pages: %w", err)
	}
	return nil
}

func discoverLocalizedCollections(site *Site, discover func(string, string, string, string) error) error {
	collectionNames := make([]string, 0, len(site.cfg.Collections))
	for name := range site.cfg.Collections {
		collectionNames = append(collectionNames, name)
	}
	sort.Strings(collectionNames)
	for _, name := range collectionNames {
		if err := discoverLocalizedCollection(site, "_"+name, name, discover); err != nil {
			return err
		}
	}
	if !site.cfg.Drafts {
		return nil
	}
	return discoverLocalizedCollection(site, "_drafts", "posts", discover)
}

func discoverLocalizedCollection(site *Site, directory, namespace string, discover func(source, relativePath, namespace, typename string) error) error {
	root := filepath.Join(site.SourceDir(), directory)
	err := filepath.Walk(root, func(filename string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}
		relativePath, err := filepath.Rel(site.SourceDir(), filename)
		if err != nil {
			return err
		}
		relativePath = filepath.ToSlash(relativePath)
		if site.Exclude(relativePath) {
			return nil
		}
		return discover(filename, relativePath, namespace, namespace)
	})
	if err != nil {
		return fmt.Errorf("discovering localized collection %q: %w", namespace, err)
	}
	return nil
}

func localizedDocumentEligible(document localization.Document, cfg config.Config) bool {
	if !document.Included || document.Namespace != "posts" {
		return document.Included
	}
	filename := filepath.Base(document.RelativePath)
	date, _, dated := utils.ParseFilenameDateTitle(filename)
	if strings.HasPrefix(document.RelativePath, "_drafts/") {
		if !cfg.Drafts {
			return false
		}
	} else if !dated {
		return false
	}
	return cfg.Future || !dated || !date.After(time.Now())
}

type routeOwner struct {
	locale         string
	namespace      string
	translationKey string
	source         string
}

func (o routeOwner) String() string {
	source := o.source
	if source == "" {
		source = "<generated>"
	}
	if o.translationKey == "" {
		return fmt.Sprintf("locale %q namespace %q source %q", o.locale, o.namespace, source)
	}
	return fmt.Sprintf("locale %q namespace %q translation key %q source %q", o.locale, o.namespace, o.translationKey, source)
}

type routeCandidate struct {
	id          int
	route       string
	destination string
	owner       routeOwner
}

// validateRoutes validates all registered output candidates rather than the
// route map. The map intentionally remains the serving lookup, but it cannot
// retain a document that a later registration would overwrite.
func (p *localizationProject) validateRoutes() error {
	candidates := p.routeCandidates(sitemapEditions(p.catalog))
	sortRouteCandidates(candidates)
	problems := append(routeCollisionProblems(candidates), destinationCollisionProblems(candidates)...)
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("invalid localized routes:\n - %s", strings.Join(problems, "\n - "))
}

func (p *localizationProject) routeCandidates(editions map[string]localization.Edition) []routeCandidate {
	candidates := make([]routeCandidate, 0)
	for _, site := range p.prepared {
		for _, document := range site.outputCandidates {
			candidate, included := localizedDocumentCandidate(len(candidates), site, document, editions)
			if included {
				candidates = append(candidates, candidate)
			}
		}
	}
	for _, aggregate := range p.aggregateRoutes {
		candidates = append(candidates, routeCandidate{
			id:          len(candidates),
			route:       aggregate.Route,
			destination: localizedRouteDestination(aggregate.Route),
			owner:       routeOwner{locale: "project", namespace: "aggregate", source: aggregate.Source},
		})
	}
	return candidates
}

func localizedDocumentCandidate(id int, site *Site, document Document, editions map[string]localization.Edition) (routeCandidate, bool) {
	if _, removed := site.removedRoutes[document.URL()]; removed {
		return routeCandidate{}, false
	}
	return routeCandidate{
		id:          id,
		route:       document.URL(),
		destination: localizedDestinationPath(document),
		owner:       localizedRouteOwner(site, document, editions[document.Source()]),
	}, true
}

func localizedRouteOwner(site *Site, document Document, edition localization.Edition) routeOwner {
	namespace := "generated"
	if document.IsStatic() {
		namespace = "static"
	}
	if edition.Source != "" {
		namespace = edition.Namespace
	}
	return routeOwner{
		locale:         site.localeKey,
		namespace:      namespace,
		translationKey: edition.TranslationKey,
		source:         document.Source(),
	}
}

func sortRouteCandidates(candidates []routeCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if left.route != right.route {
			return left.route < right.route
		}
		if left.owner.locale != right.owner.locale {
			return left.owner.locale < right.owner.locale
		}
		if left.owner.namespace != right.owner.namespace {
			return left.owner.namespace < right.owner.namespace
		}
		if left.owner.translationKey != right.owner.translationKey {
			return left.owner.translationKey < right.owner.translationKey
		}
		return left.owner.source < right.owner.source
	})
}

func routeCollisionProblems(candidates []routeCandidate) []string {
	publicRoutes := make(map[string]routeCandidate)
	var problems []string
	for _, candidate := range candidates {
		for _, route := range localizedRouteAliases(candidate.route) {
			if previous, exists := publicRoutes[route]; exists && previous.id != candidate.id {
				problems = append(problems, fmt.Sprintf("route collision at %q between %s and %s", route, previous.owner, candidate.owner))
				continue
			}
			publicRoutes[route] = candidate
		}
	}
	return problems
}

func destinationCollisionProblems(candidates []routeCandidate) []string {
	destinations := make(map[string]routeCandidate)
	var problems []string
	for _, candidate := range candidates {
		if previous, exists := destinations[candidate.destination]; exists && previous.id != candidate.id {
			problems = append(problems, fmt.Sprintf("destination collision at %q between %s and %s", candidate.destination, previous.owner, candidate.owner))
			continue
		}
		destinations[candidate.destination] = candidate
	}
	return problems
}

func localizedDestinationPath(document Document) string {
	if document.IsStatic() {
		return localizedRouteDestination(document.URL())
	}
	return localizedRouteDestination(destinationRelativePath(document))
}

func localizedRouteDestination(route string) string {
	destination := filepath.Clean(strings.TrimPrefix(route, "/"))
	if filepath.Ext(destination) == "" && !strings.HasSuffix(route, "/") {
		destination = filepath.Join(destination, "index.html")
	}
	if strings.HasSuffix(route, "/") && filepath.Ext(destination) == "" {
		destination = filepath.Join(destination, "index.html")
	}
	return filepath.ToSlash(destination)
}

func localizedRouteAliases(route string) []string {
	aliases := map[string]struct{}{route: {}}
	switch {
	case strings.HasSuffix(route, "/"):
		aliases[route+"index.html"] = struct{}{}
		aliases[route+"index.htm"] = struct{}{}
	case strings.HasSuffix(route, "index.html"):
		prefix := strings.TrimSuffix(route, "index.html")
		aliases[prefix] = struct{}{}
		aliases[strings.TrimSuffix(prefix, "/")] = struct{}{}
	case strings.HasSuffix(route, "index.htm"):
		prefix := strings.TrimSuffix(route, "index.htm")
		aliases[prefix] = struct{}{}
		aliases[strings.TrimSuffix(prefix, "/")] = struct{}{}
	}
	if strings.HasSuffix(route, ".html") {
		aliases[strings.TrimSuffix(route, ".html")] = struct{}{}
	}
	out := make([]string, 0, len(aliases))
	for alias := range aliases {
		out = append(out, alias)
	}
	sort.Strings(out)
	return out
}

func copyKeepFiles(source, destination string, keepFiles []string) error {
	for _, keep := range keepFiles {
		from := filepath.Join(source, keep)
		info, err := os.Stat(from)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if err := copyLocalizedPath(from, filepath.Join(destination, keep), info); err != nil {
			return err
		}
	}
	return nil
}

func copyLocalizedPath(source, destination string, info os.FileInfo) error {
	if !info.IsDir() {
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		return utils.CopyFileContents(destination, source, info.Mode())
	}
	return filepath.Walk(source, func(path string, entry os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, entry.Mode())
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return utils.CopyFileContents(target, path, entry.Mode())
	})
}

func promoteLocalizedGeneration(stage, destination string) error {
	backup := destination + ".jigyll-localized-backup"
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("clearing localized promotion backup: %w", err)
	}

	destinationExists := false
	if _, err := os.Stat(destination); err == nil {
		destinationExists = true
		if err := os.Rename(destination, backup); err != nil {
			return fmt.Errorf("backing up current destination: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	restore := func() error {
		if !destinationExists {
			return nil
		}
		if err := os.RemoveAll(destination); err != nil {
			return err
		}
		return os.Rename(backup, destination)
	}
	if err := os.Rename(stage, destination); err != nil {
		if restoreErr := restore(); restoreErr != nil {
			return fmt.Errorf("promoting localized generation: %w; restoring previous destination: %v", err, restoreErr)
		}
		return fmt.Errorf("promoting localized generation: %w", err)
	}
	if destinationExists {
		if err := os.RemoveAll(backup); err != nil {
			if restoreErr := restore(); restoreErr != nil {
				return fmt.Errorf("removing localized promotion backup: %w; restoring previous destination: %v", err, restoreErr)
			}
			return fmt.Errorf("removing localized promotion backup: %w", err)
		}
	}
	return nil
}
