package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	mathrand "math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/antiscale/backend/models"
	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"gorm.io/gorm"
)

var db *gorm.DB

var flowers = []string{
	"peony", "dahlia", "tulip", "orchid", "rose", "lily", "daisy",
	"iris", "poppy", "violet", "lotus", "aster", "jasmine", "lilac",
}

func generateMagicName(hostname string) string {
	f1 := flowers[mathrand.Intn(len(flowers))]
	f2 := flowers[mathrand.Intn(len(flowers))]
	return fmt.Sprintf("%s-%s", f1, f2)
}

func generateCGNATIP(count int64) string {
	y := (count + 2) / 254
	z := (count + 2) % 254
	return fmt.Sprintf("100.%d.%d.%d", 64+(y/256), y%256, z+1)
}

func initDB() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "antiscale.db"
	}

	var err error
	db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	err = db.AutoMigrate(&models.Node{}, &models.ACLPolicy{}, &models.User{}, &models.AuthKey{})
	if err != nil {
		log.Fatal("Failed to migrate database:", err)
	}
	
	var count int64
	db.Model(&models.ACLPolicy{}).Count(&count)
	if count == 0 {
		defaultACL := `{
  "groups": {},
  "acls": [
    { "action": "accept", "src": ["*"], "dst": ["*:*"] }
  ]
}`
		db.Create(&models.ACLPolicy{Policy: defaultACL})
	}
}

// Session store wrapping User IDs with expiration timestamps to prevent memory leaks
type SessionData struct {
	UserID    uint
	ExpiresAt time.Time
}
var Sessions = make(map[string]SessionData)

// Middleware to secure Admin endpoints
func authRequired(c *fiber.Ctx) error {
	if os.Getenv("GITHUB_CLIENT_ID") == "" || os.Getenv("DEV_MODE") == "true" {
		var user models.User
		if err := db.Where("username = ?", "Dev Admin").First(&user).Error; err != nil {
			user = models.User{Username: "Dev Admin", GithubID: 0, AvatarURL: "https://github.com/github.png"}
			db.Create(&user)
		}
		c.Locals("userID", user.ID)
		return c.Next()
	}

	session := c.Cookies("antiscale_session")
	data, ok := Sessions[session]
	if !ok || time.Now().After(data.ExpiresAt) {
		if ok { delete(Sessions, session) } // Clean up expired
		return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
	}
	// Store UserID in context for handlers to use
	c.Locals("userID", data.UserID)
	return c.Next()
}

func main() {
	mathrand.Seed(time.Now().UnixNano())
	initDB()

	// Memory Leak Garbage Collector
	go func() {
		for {
			time.Sleep(1 * time.Hour)
			now := time.Now()
			for k, v := range Sessions {
				if now.After(v.ExpiresAt) {
					delete(Sessions, k)
				}
			}
		}
	}()

	app := fiber.New(fiber.Config{
		AppName: "Antiscale Controller",
	})

	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOriginsFunc: func(origin string) bool { return true },
		AllowCredentials: true,
		AllowHeaders:     "Origin, Content-Type, Accept",
	}))

	// --- API Routes ---
	api := app.Group("/api")
	
	// Node Registration & Sync (Public / Pre-AuthKey Handled)
	api.Post("/register", registerNode)
	api.Get("/sync/:public_key", syncPeers)

	// Auth (GitHub SSO)
	api.Get("/auth/github", githubLogin)
	api.Get("/auth/github/callback", githubCallback)
	api.Get("/auth/me", getMe)

	// Admin Dashboard (Protected by strict Middleware)
	admin := api.Group("/", authRequired)
	admin.Get("/nodes", getNodes)
	admin.Put("/nodes/:id/approve", approveNode)
	admin.Delete("/nodes/:id", deleteNode)
	admin.Put("/nodes/:id/routes", updateNodeRoutes)
	admin.Get("/acl", getACL)
	admin.Put("/acl", updateACL)
	admin.Get("/auth_keys", getAuthKeys)
	admin.Post("/auth_keys", createAuthKey)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Fatal(app.Listen(":" + port))
}

// ---------------------------
// GITHUB SSO FLOW
// ---------------------------

func githubLogin(c *fiber.Ctx) error {
	clientID := os.Getenv("GITHUB_CLIENT_ID")
	redirectURI := c.BaseURL() + "/api/auth/github/callback"
	url := fmt.Sprintf("https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=read:user", clientID, redirectURI)
	return c.Redirect(url)
}

func githubCallback(c *fiber.Ctx) error {
	code := c.Query("code")
	clientID := os.Getenv("GITHUB_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_CLIENT_SECRET")

	reqBody, _ := json.Marshal(map[string]string{
		"client_id":     clientID,
		"client_secret": clientSecret,
		"code":          code,
	})
	
	req, _ := http.NewRequest("POST", "https://github.com/login/oauth/access_token", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil { return c.Status(500).SendString("Oauth Exchange Failed") }
	defer resp.Body.Close()

	var tokenRes struct { AccessToken string `json:"access_token"` }
	json.NewDecoder(resp.Body).Decode(&tokenRes)

	reqUser, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	reqUser.Header.Set("Authorization", "Bearer "+tokenRes.AccessToken)
	
	respUser, _ := client.Do(reqUser)
	defer respUser.Body.Close()

	var ghUser struct { ID int64 `json:"id"`; Login string `json:"login"`; AvatarURL string `json:"avatar_url"` }
	json.NewDecoder(respUser.Body).Decode(&ghUser)

	var user models.User
	if err := db.Where("github_id = ?", ghUser.ID).First(&user).Error; err != nil {
		user = models.User{ GithubID: ghUser.ID, Username: ghUser.Login, AvatarURL: ghUser.AvatarURL }
		db.Create(&user)
	}

	// Cryptographically secure session token
	b := make([]byte, 32)
	rand.Read(b)
	sessionToken := fmt.Sprintf("sess_%x", b)
	
	Sessions[sessionToken] = SessionData{
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	c.Cookie(&fiber.Cookie{
		Name:     "antiscale_session",
		Value:    sessionToken,
		Expires:  time.Now().Add(24 * time.Hour),
		HTTPOnly: true,
		Secure:   false, // Keep false for localhost HTTP MVP, set true in prod
		Path:     "/",
	})

	host := strings.Split(c.Hostname(), ":")[0]
	return c.Redirect(c.Protocol() + "://" + host + "/")
}

func getMe(c *fiber.Ctx) error {
	if os.Getenv("GITHUB_CLIENT_ID") == "" || os.Getenv("DEV_MODE") == "true" {
		var user models.User
		db.Where("username = ?", "Dev Admin").First(&user)
		return c.JSON(user)
	}

	session := c.Cookies("antiscale_session")
	data, ok := Sessions[session]
	if !ok || time.Now().After(data.ExpiresAt) {
		return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
	}
	var user models.User
	db.First(&user, data.UserID)
	return c.JSON(user)
}

// ---------------------------
// DEVICE REGISTRATION API
// ---------------------------

func registerNode(c *fiber.Ctx) error {
	type ConnectRequest struct {
		Hostname         string `json:"hostname"`
		PublicKey        string `json:"public_key"`
		AdvertisedRoutes string `json:"advertised_routes"`
		AcceptRoutes     bool   `json:"accept_routes"`
		AuthKey          string `json:"auth_key"`
	}

	req := new(ConnectRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	var node models.Node
	status := "pending"
	var ownerID uint = 0
	
	if req.AuthKey != "" {
		var key models.AuthKey
		if err := db.Where("key = ?", req.AuthKey).First(&key).Error; err == nil {
			if !key.IsReusable && key.IsUsed {
				return c.Status(403).JSON(fiber.Map{"error": "AuthKey already used"})
			}
			ownerID = key.UserID
			if key.AutoApprove { status = "approved" }
			if !key.IsReusable {
				key.IsUsed = true
				db.Save(&key)
			}
		} else {
			return c.Status(403).JSON(fiber.Map{"error": "Invalid AuthKey"})
		}
	}

	endpoint := fmt.Sprintf("%s:51820", c.IP())

	result := db.Where("public_key = ?", req.PublicKey).First(&node)
	if result.Error != nil {
		var count int64
		db.Model(&models.Node{}).Count(&count)
		newIP := generateCGNATIP(count)
		magic := generateMagicName(req.Hostname)

		node = models.Node{
			UserID: ownerID, Hostname: req.Hostname, MagicName: magic,
			PublicKey: req.PublicKey, PrivateIP: newIP,
			AdvertisedRoutes: req.AdvertisedRoutes, AcceptRoutes: req.AcceptRoutes, Status: status,
			Endpoint: endpoint,
		}
		db.Create(&node)
	} else {
		node.AdvertisedRoutes = req.AdvertisedRoutes
		node.AcceptRoutes = req.AcceptRoutes
		node.Hostname = req.Hostname
		node.Endpoint = endpoint
		if ownerID != 0 { node.UserID = ownerID }
		if status == "approved" && node.Status == "pending" { node.Status = "approved" }
		db.Save(&node)
	}

	return c.JSON(node)
}

func syncPeers(c *fiber.Ctx) error {
	pubKey := c.Params("public_key")
	var me models.Node
	if err := db.Where("public_key = ?", pubKey).First(&me).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "node not found"})
	}
	if me.Status != "approved" {
		return c.Status(403).JSON(fiber.Map{"error": "node not approved"})
	}

	// Fetch Policy
	var acl models.ACLPolicy
	db.First(&acl)
	var policyMap map[string]interface{}
	json.Unmarshal([]byte(acl.Policy), &policyMap)
	// (For MVP, assuming * allows all. Tailscale's actual parsing is vastly more complex)

	var peers []models.Node
	db.Where("status = ? AND public_key != ?", "approved", pubKey).Find(&peers)
	
	// Future ACL filtering logic goes here by omitting peers from the array
	// if they don't match ACL. For now, it passes all approved peers.

	return c.JSON(peers)
}

// ---------------------------
// ADMIN CONTROLS API (PROTECTED)
// ---------------------------

func getNodes(c *fiber.Ctx) error {
	var nodes []models.Node
	db.Find(&nodes)
	return c.JSON(nodes)
}

func approveNode(c *fiber.Ctx) error {
	var node models.Node
	if err := db.First(&node, c.Params("id")).Error; err != nil { return c.Status(404).JSON(fiber.Map{"error": "node not found"}) }
	node.Status = "approved"
	db.Save(&node)
	return c.JSON(node)
}

func deleteNode(c *fiber.Ctx) error {
	if err := db.Delete(&models.Node{}, c.Params("id")).Error; err != nil { return c.Status(500).JSON(fiber.Map{"error": "delete failed"}) }
	return c.SendStatus(204)
}

func updateNodeRoutes(c *fiber.Ctx) error {
	type RouteReq struct { ApprovedRoutes string `json:"approved_routes"` }
	req := new(RouteReq)
	c.BodyParser(req)
	var node models.Node
	if err := db.First(&node, c.Params("id")).Error; err != nil { return c.Status(404).JSON(fiber.Map{"error": "node not found"}) }
	node.ApprovedRoutes = req.ApprovedRoutes
	db.Save(&node)
	return c.JSON(node)
}

func getACL(c *fiber.Ctx) error {
	var acl models.ACLPolicy
	db.First(&acl)
	return c.JSON(acl)
}

func updateACL(c *fiber.Ctx) error {
	var acl models.ACLPolicy
	db.First(&acl)
	type ACLReq struct { Policy string `json:"policy"` }
	req := new(ACLReq)
	c.BodyParser(req)
	acl.Policy = req.Policy
	db.Save(&acl)
	return c.JSON(acl)
}

func getAuthKeys(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	var keys []models.AuthKey
	db.Where("user_id = ?", userID).Find(&keys)
	return c.JSON(keys)
}

func createAuthKey(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)

	type KeyReq struct { AutoApprove bool `json:"auto_approve"` }
	req := new(KeyReq)
	c.BodyParser(req)

	b := make([]byte, 16)
	rand.Read(b)
	secureKey := fmt.Sprintf("antskey-%x", b)

	key := models.AuthKey{
		UserID:      userID,
		Key:         secureKey,
		AutoApprove: req.AutoApprove,
		IsReusable:  true,
	}
	db.Create(&key)
	return c.JSON(key)
}
