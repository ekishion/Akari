package admin

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"akari-bridge/internal/domain"
	"akari-bridge/internal/engine"
	"akari-bridge/internal/rules"
)

type RuleController struct {
	ruleMgr  *rules.RuleManager
	eng      *engine.Engine
	adminSvc domain.IAdminService
}

func NewRuleController(ruleMgr *rules.RuleManager, eng *engine.Engine, adminSvc domain.IAdminService) *RuleController {
	return &RuleController{
		ruleMgr:  ruleMgr,
		eng:      eng,
		adminSvc: adminSvc,
	}
}

func (ctrl *RuleController) ListRules(c *gin.Context) {
	allRules := ctrl.ruleMgr.GetAllPlugins()
	c.JSON(http.StatusOK, allRules)
}

func (ctrl *RuleController) GetRule(c *gin.Context) {
	name := c.Param("name")
	if unescaped, err := url.PathUnescape(name); err == nil && unescaped != "" {
		name = unescaped
	}
	p, ok := ctrl.ruleMgr.GetPluginByName(name)
	if !ok || p == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (ctrl *RuleController) SaveRule(c *gin.Context) {
	var plugin engine.Plugin
	if err := c.ShouldBindJSON(&plugin); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid plugin JSON format"})
		return
	}

	if err := ctrl.ruleMgr.SavePlugin(&plugin); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, plugin)
}

func (ctrl *RuleController) ToggleRule(c *gin.Context) {
	name := c.Param("name")
	if unescaped, err := url.PathUnescape(name); err == nil && unescaped != "" {
		name = unescaped
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	if err := ctrl.ruleMgr.TogglePlugin(name, req.Enabled); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "name": name, "enabled": req.Enabled})
}

func (ctrl *RuleController) DeleteRule(c *gin.Context) {
	name := c.Param("name")
	if unescaped, err := url.PathUnescape(name); err == nil && unescaped != "" {
		name = unescaped
	}
	if err := ctrl.ruleMgr.DeletePlugin(name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (ctrl *RuleController) ImportRulesFromURL(c *gin.Context) {
	var req struct {
		URL string `json:"url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL is required"})
		return
	}

	stats, err := ctrl.ruleMgr.ImportPluginsFromURL(req.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "ok",
		"importedCount": stats.TotalCount,
		"addedCount":    stats.AddedCount,
		"updatedCount":  stats.UpdatedCount,
	})
}

func (ctrl *RuleController) UpdateAllRules(c *gin.Context) {
	var req struct {
		URL string `json:"url"`
	}
	_ = c.ShouldBindJSON(&req)
	targetURL := strings.TrimSpace(req.URL)
	if targetURL == "" {
		targetURL = "https://raw.githubusercontent.com/Predidit/KazumiRules/master/index.json"
	}

	stats, err := ctrl.ruleMgr.ImportPluginsFromURL(targetURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "ok",
		"importedCount": stats.TotalCount,
		"addedCount":    stats.AddedCount,
		"updatedCount":  stats.UpdatedCount,
	})
}

type TestRuleReq struct {
	Plugin   *engine.Plugin `json:"plugin,omitempty"`
	RuleName string         `json:"ruleName,omitempty"`
	Keyword  string         `json:"keyword"`
}

func (ctrl *RuleController) TestRule(c *gin.Context) {
	var req TestRuleReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Keyword is required"})
		return
	}

	plugin := req.Plugin
	if plugin == nil && req.RuleName != "" {
		p, found := ctrl.ruleMgr.GetPluginByName(req.RuleName)
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "Rule not found"})
			return
		}
		plugin = p
	}

	if plugin == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No plugin provided"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results, err := ctrl.eng.Search(ctx, plugin, req.Keyword)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   err.Error(),
			"results": []engine.SearchItem{},
		})
		return
	}

	var chapters []engine.Road
	var chapterErr string
	sampleDrama := ""
	if len(results) > 0 && results[0].Src != "" {
		sampleDrama = results[0].Src
		chList, err := ctrl.eng.QueryChapters(ctx, plugin, results[0].Src)
		if err != nil {
			chapterErr = err.Error()
		} else {
			chapters = chList
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"resultsCount": len(results),
		"results":      results,
		"sampleDrama":  sampleDrama,
		"chapters":     chapters,
		"chapterError": chapterErr,
	})
}
