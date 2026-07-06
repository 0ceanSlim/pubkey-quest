package auth

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/client/session"
	"pubkey-quest/cmd/server/utils"
)

// AuthHandler handles all authentication-related operations using grain client
type AuthHandler struct {
	config *utils.Config
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(cfg *utils.Config) *AuthHandler {
	return &AuthHandler{config: cfg}
}

// LoginRequest represents a login request for Pubkey Quest.
//
// As of grain 0.8 all signing happens client-side (mill installs a
// window.nostr-compatible signer). The server never receives a private key —
// it only records the authenticated hex public key and the signing method the
// client chose.
type LoginRequest struct {
	PublicKey     string                         `json:"public_key,omitempty"`
	SigningMethod session.SigningMethod          `json:"signing_method"`
	Mode          session.SessionInteractionMode `json:"mode,omitempty"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Success     bool                `json:"success"`
	Message     string              `json:"message"`
	Session     *session.UserSession `json:"session,omitempty"`
	PublicKey   string              `json:"public_key,omitempty"`
	NPub        string              `json:"npub,omitempty"`
	Error       string              `json:"error,omitempty"`
}

// SessionResponse represents a session status response
type SessionResponse struct {
	Success  bool                `json:"success"`
	IsActive bool                `json:"is_active"`
	Session  *session.UserSession `json:"session,omitempty"`
	NPub     string              `json:"npub,omitempty"`
	Error    string              `json:"error,omitempty"`
}

// HandleLogin handles user login/authentication using grain session system
func (auth *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request
	if err := auth.validateLoginRequest(&req); err != nil {
		auth.sendErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Default to write mode for game functionality
	if req.Mode == "" {
		req.Mode = session.WriteMode
	}

	// The client (mill) does all signing and only ever sends us the resulting
	// public key. Accept npub or hex and normalize to hex.
	if req.PublicKey == "" {
		auth.sendErrorResponse(w, "Public key is required", http.StatusBadRequest)
		return
	}

	pubkeyHex, err := normalizePubkey(req.PublicKey)
	if err != nil {
		auth.sendErrorResponse(w, fmt.Sprintf("Invalid public key: %v", err), http.StatusBadRequest)
		return
	}

	// Create session init request for grain
	sessionReq := session.SessionInitRequest{
		RequestedMode: req.Mode,
		SigningMethod: req.SigningMethod,
		PublicKey:     pubkeyHex,
	}

	// Check whitelist access before creating session
	if err := auth.checkWhitelistAccess(r, sessionReq.PublicKey); err != nil {
		if whitelistErr, ok := err.(*WhitelistError); ok {
			// Send whitelist-specific error with form URL
			auth.sendWhitelistErrorResponse(w, whitelistErr)
		} else {
			auth.sendErrorResponse(w, err.Error(), http.StatusForbidden)
		}
		return
	}

	// Create user session using grain
	userSession, err := session.CreateUserSession(w, sessionReq)
	if err != nil {
		auth.sendErrorResponse(w, fmt.Sprintf("Failed to create session: %v", err), http.StatusBadRequest)
		return
	}

	// Generate npub for response
	npub, _ := tools.EncodePubkey(userSession.PublicKey)

	log.Printf("🎮 Pubkey Quest user logged in: %s (%s mode)", userSession.PublicKey[:16]+"...", userSession.Mode)

	response := LoginResponse{
		Success:   true,
		Message:   "Login successful",
		Session:   userSession,
		PublicKey: userSession.PublicKey,
		NPub:      npub,
	}

	auth.sendJSONResponse(w, response, http.StatusOK)
}

// HandleLogout handles user logout
func (auth *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Clear session using grain session manager
	if session.SessionMgr != nil {
		session.SessionMgr.ClearSession(w, r)
	}

	log.Println("🎮 Pubkey Quest user logged out")

	response := map[string]interface{}{
		"success": true,
		"message": "Logged out successfully",
	}

	auth.sendJSONResponse(w, response, http.StatusOK)
}

// HandleSession handles session status checks
func (auth *AuthHandler) HandleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get session from grain session manager
	if !session.IsSessionManagerInitialized() {
		response := SessionResponse{
			Success:  true,
			IsActive: false,
			Error:    "session manager not initialized",
		}
		auth.sendJSONResponse(w, response, http.StatusOK)
		return
	}

	userSession := session.SessionMgr.GetCurrentUser(r)
	if userSession == nil {
		response := SessionResponse{
			Success:  true,
			IsActive: false,
		}
		auth.sendJSONResponse(w, response, http.StatusOK)
		return
	}

	// Generate npub for response
	npub, _ := tools.EncodePubkey(userSession.PublicKey)

	response := SessionResponse{
		Success:  true,
		IsActive: true,
		Session:  userSession,
		NPub:     npub,
	}

	auth.sendJSONResponse(w, response, http.StatusOK)
}

// HandleAmberCallback receives Amber's NIP-55 redirect and hands the resulting
// public key back to the MILL modal running in the user's original tab. It
// speaks MILL's amber-callback protocol (localStorage key "mill:amber:result"
// plus a postMessage to window.opener); MILL's awaitAmberResult listener there
// completes the connection, and the resulting pubkey then flows through the
// normal HandleLogin path — which is where the session + whitelist check happen.
func (auth *AuthHandler) HandleAmberCallback(w http.ResponseWriter, r *http.Request) {
	log.Printf("🎮 Amber callback received: url=%s", r.URL.String())

	eventParam := r.URL.Query().Get("event")

	var pubkeyHex, errMsg string
	if eventParam == "" {
		log.Printf("❌ Amber callback missing event parameter")
		errMsg = "Missing event data from Amber"
	} else {
		// Amber may URL-encode the event parameter.
		decodedEvent, decErr := url.QueryUnescape(eventParam)
		if decErr != nil {
			decodedEvent = eventParam
		}

		pk, err := auth.extractPublicKeyFromAmber(decodedEvent)
		if err != nil {
			log.Printf("❌ Failed to extract public key from amber response: %v", err)
			errMsg = "Invalid response from Amber"
		} else {
			pubkeyHex = pk
			log.Printf("✅ Amber callback resolved pubkey: %s...", pubkeyHex[:16])
		}
	}

	auth.renderAmberRelay(w, pubkeyHex, errMsg)
}

// renderAmberRelay returns a minimal page that writes the Amber result into
// MILL's amber-callback channel and closes itself. It intentionally does not
// create a session — that happens uniformly in HandleLogin once the MILL modal
// reports the connection.
func (auth *AuthHandler) renderAmberRelay(w http.ResponseWriter, pubkeyHex, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	// Marshal to JS string literals for safe embedding.
	eventJSON, _ := json.Marshal(pubkeyHex)
	errorJSON, _ := json.Marshal(errMsg)

	html := `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Amber — Pubkey Quest</title>
	<style>
		body { font-family: 'Dogica Pixel', monospace; margin: 0; padding: 24px; background: #001100; color: #00ff41; text-align: center; }
		.err { color: #ff4444; }
	</style>
</head>
<body>
	<p id="msg">Returning to Pubkey Quest…</p>
	<script>
		(function () {
			var event = ` + string(eventJSON) + `;
			var error = ` + string(errorJSON) + `;
			try { localStorage.setItem('mill:amber:result', JSON.stringify({ event: event, error: error })); } catch (e) {}
			try { if (window.opener) window.opener.postMessage({ amberEvent: event, amberError: error }, '*'); } catch (e) {}
			if (error) {
				var m = document.getElementById('msg');
				m.className = 'err';
				m.textContent = 'Amber error: ' + error;
			}
			setTimeout(function () {
				try {
					if (window.opener && !window.opener.closed) { window.close(); }
					else { window.location.href = '/'; }
				} catch (e) { window.location.href = '/'; }
			}, 400);
		})();
	</script>
</body>
</html>`

	w.Write([]byte(html))
}

// GetCurrentUser is a utility function to get the current authenticated user
func GetCurrentUser(r *http.Request) *session.UserSession {
	if !session.IsSessionManagerInitialized() {
		return nil
	}
	return session.SessionMgr.GetCurrentUser(r)
}

// Helper methods

func (auth *AuthHandler) validateLoginRequest(req *LoginRequest) error {
	if req.SigningMethod == "" {
		return fmt.Errorf("signing method is required")
	}

	if req.Mode == "" {
		req.Mode = session.WriteMode // Default to write mode for game
	}

	return nil
}

func (auth *AuthHandler) extractPublicKeyFromAmber(eventParam string) (string, error) {
	// Handle compressed response (starts with "Signer1")
	if strings.HasPrefix(eventParam, "Signer1") {
		return "", fmt.Errorf("compressed Amber responses not supported")
	}

	// Try to parse as JSON event (returnType=signature or returnType=event)
	var event map[string]interface{}
	if err := json.Unmarshal([]byte(eventParam), &event); err == nil {
		// It's a JSON event, extract pubkey field
		if pubkey, ok := event["pubkey"].(string); ok && len(pubkey) == 64 {
			log.Printf("✅ Extracted pubkey from JSON event: %s", pubkey)
			return pubkey, nil
		}

		// Check if it's wrapped in an "event" field
		if eventObj, ok := event["event"].(map[string]interface{}); ok {
			if pubkey, ok := eventObj["pubkey"].(string); ok && len(pubkey) == 64 {
				log.Printf("✅ Extracted pubkey from nested event: %s", pubkey)
				return pubkey, nil
			}
		}

		log.Printf("⚠️ JSON event doesn't have valid pubkey field: %v", event)
		return "", fmt.Errorf("event JSON missing valid pubkey field")
	}

	// Fallback: treat as direct public key string
	publicKey := strings.TrimSpace(eventParam)

	// Validate public key format (64 hex characters)
	pubKeyRegex := regexp.MustCompile(`^[a-fA-F0-9]{64}$`)
	if !pubKeyRegex.MatchString(publicKey) {
		log.Printf("⚠️ Not a valid hex pubkey: %s", publicKey)
		return "", fmt.Errorf("invalid public key format from Amber")
	}

	log.Printf("✅ Extracted pubkey as direct string: %s", publicKey)
	return publicKey, nil
}

// isLocalConnection checks if the request is from localhost or local network
func isLocalConnection(r *http.Request) bool {
	// Get the client IP from the request
	clientIP := getClientIP(r)

	// Parse the IP
	ip := net.ParseIP(clientIP)
	if ip == nil {
		log.Printf("⚠️ Failed to parse client IP: %s", clientIP)
		return false
	}

	// Check if it's localhost (IPv4 or IPv6)
	if ip.IsLoopback() {
		log.Printf("🔓 Connection from localhost: %s", clientIP)
		return true
	}

	// Check if it's a private network address (192.168.x.x, 10.x.x.x, 172.16-31.x.x)
	if ip.IsPrivate() {
		log.Printf("🔓 Connection from local network: %s", clientIP)
		return true
	}

	log.Printf("🌐 Connection from external IP: %s", clientIP)
	return false
}

// getClientIP extracts the client IP from the request, handling proxies
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// normalizePubkey converts npub to hex or returns hex unchanged
func normalizePubkey(pubkey string) (string, error) {
	// If it's already hex (64 characters)
	if len(pubkey) == 64 {
		if matched, _ := regexp.MatchString("^[0-9a-fA-F]{64}$", pubkey); matched {
			return strings.ToLower(pubkey), nil
		}
	}

	// If it's npub format, decode it
	if strings.HasPrefix(pubkey, "npub") {
		hexPubkey, err := tools.DecodeNpub(pubkey)
		if err != nil {
			return "", fmt.Errorf("failed to decode npub: %w", err)
		}
		return strings.ToLower(hexPubkey), nil
	}

	return "", fmt.Errorf("invalid pubkey format: must be npub or 64-char hex")
}

// isWhitelisted checks if a pubkey is in the whitelist
func (auth *AuthHandler) isWhitelisted(pubkey string) bool {
	// If no whitelist is configured, allow all
	if len(auth.config.Server.Whitelist) == 0 {
		return true
	}

	// Normalize the pubkey to check
	normalizedPubkey, err := normalizePubkey(pubkey)
	if err != nil {
		log.Printf("⚠️ Failed to normalize pubkey for whitelist check: %v", err)
		return false
	}

	// Check against each whitelisted key
	for _, whitelistedKey := range auth.config.Server.Whitelist {
		normalizedWhitelisted, err := normalizePubkey(whitelistedKey)
		if err != nil {
			log.Printf("⚠️ Invalid whitelist entry: %s - %v", whitelistedKey, err)
			continue
		}

		if normalizedPubkey == normalizedWhitelisted {
			return true
		}
	}

	return false
}

// WhitelistError represents a whitelist access denial. Access is now requested
// through the in-app form (POST /api/report/access), so no external form URL is
// carried here. Pubkey is the denied key (hex), surfaced to the client so the
// access-request form can auto-fill it.
type WhitelistError struct {
	Message string
	Pubkey  string
}

func (e *WhitelistError) Error() string {
	return e.Message
}

// checkWhitelistAccess checks if a pubkey should be allowed based on whitelist rules
func (auth *AuthHandler) checkWhitelistAccess(r *http.Request, pubkey string) error {
	// Whitelist is only enforced when debug mode is enabled
	if !auth.config.Server.DebugMode {
		return nil
	}

	// Allow all connections from localhost or local network
	if isLocalConnection(r) {
		return nil
	}

	// Check whitelist for external connections
	if !auth.isWhitelisted(pubkey) {
		log.Printf("🚫 Access denied for non-whitelisted pubkey: %s...", pubkey[:16])
		return &WhitelistError{
			Message: "Access denied: Your public key is not whitelisted for this test server",
			Pubkey:  pubkey,
		}
	}

	log.Printf("✅ Whitelisted pubkey allowed: %s...", pubkey[:16])
	return nil
}

func (auth *AuthHandler) sendJSONResponse(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func (auth *AuthHandler) sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	response := map[string]interface{}{
		"success": false,
		"error":   message,
	}
	auth.sendJSONResponse(w, response, statusCode)
}

func (auth *AuthHandler) sendWhitelistErrorResponse(w http.ResponseWriter, whitelistErr *WhitelistError) {
	npub, _ := tools.EncodePubkey(whitelistErr.Pubkey)
	response := map[string]interface{}{
		"success":          false,
		"error":            whitelistErr.Message,
		"whitelist_denial": true,
		"npub":             npub,
	}
	auth.sendJSONResponse(w, response, http.StatusForbidden)
}