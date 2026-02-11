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

// LoginRequest represents a login request for Pubkey Quest
type LoginRequest struct {
	PublicKey     string                         `json:"public_key,omitempty"`
	PrivateKey    string                         `json:"private_key,omitempty"`  // nsec format
	SigningMethod session.SigningMethod         `json:"signing_method"`
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

// KeyPairResponse represents a key generation response
type KeyPairResponse struct {
	Success bool           `json:"success"`
	KeyPair *tools.KeyPair `json:"key_pair,omitempty"`
	Error   string         `json:"error,omitempty"`
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

	// Create session init request for grain
	sessionReq := session.SessionInitRequest{
		RequestedMode: req.Mode,
		SigningMethod: req.SigningMethod,
	}

	// Handle different signing methods
	switch req.SigningMethod {
	case session.BrowserExtension:
		if req.PublicKey == "" {
			auth.sendErrorResponse(w, "Public key required for browser extension signing", http.StatusBadRequest)
			return
		}
		sessionReq.PublicKey = req.PublicKey

	case session.AmberSigning:
		if req.PublicKey == "" {
			auth.sendErrorResponse(w, "Public key required for Amber signing", http.StatusBadRequest)
			return
		}
		sessionReq.PublicKey = req.PublicKey

	case session.EncryptedKey:
		if req.PrivateKey == "" {
			auth.sendErrorResponse(w, "Private key required for encrypted key signing", http.StatusBadRequest)
			return
		}

		var privateKeyHex string
		var err error

		// Handle both nsec and hex format
		if strings.HasPrefix(req.PrivateKey, "nsec") {
			// Decode nsec to get hex private key
			privateKeyHex, err = tools.DecodeNsec(req.PrivateKey)
			if err != nil {
				auth.sendErrorResponse(w, fmt.Sprintf("Invalid nsec format: %v", err), http.StatusBadRequest)
				return
			}
		} else if len(req.PrivateKey) == 64 {
			// Assume it's already hex format
			if matched, _ := regexp.MatchString("^[0-9a-fA-F]{64}$", req.PrivateKey); !matched {
				auth.sendErrorResponse(w, "Invalid hex private key format", http.StatusBadRequest)
				return
			}
			privateKeyHex = req.PrivateKey
		} else {
			auth.sendErrorResponse(w, "Private key must be nsec format or 64-character hex", http.StatusBadRequest)
			return
		}

		pubkey, err := tools.DerivePublicKey(privateKeyHex)
		if err != nil {
			auth.sendErrorResponse(w, fmt.Sprintf("Failed to derive public key: %v", err), http.StatusBadRequest)
			return
		}

		sessionReq.PublicKey = pubkey
		sessionReq.PrivateKey = req.PrivateKey

	default:
		// For read-only mode or other cases
		if req.PublicKey != "" {
			sessionReq.PublicKey = req.PublicKey
		} else {
			auth.sendErrorResponse(w, "Either public key or private key must be provided", http.StatusBadRequest)
			return
		}
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

// HandleGenerateKeys handles key pair generation
func (auth *AuthHandler) HandleGenerateKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Generate new key pair using grain tools
	keyPair, err := tools.GenerateKeyPair()
	if err != nil {
		auth.sendErrorResponse(w, fmt.Sprintf("Failed to generate keys: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("🎮 Generated new key pair for Pubkey Quest: %s", keyPair.Npub)

	response := KeyPairResponse{
		Success: true,
		KeyPair: keyPair,
	}

	auth.sendJSONResponse(w, response, http.StatusOK)
}

// HandleAmberCallback processes callbacks from Amber app
func (auth *AuthHandler) HandleAmberCallback(w http.ResponseWriter, r *http.Request) {
	log.Printf("🎮 Amber callback received: method=%s, url=%s", r.Method, r.URL.String())

	// Parse query parameters
	eventParam := r.URL.Query().Get("event")
	if eventParam == "" {
		log.Printf("❌ Amber callback missing event parameter")
		auth.renderAmberError(w, "Missing event data from Amber")
		return
	}

	// URL decode the event parameter (Amber may send it encoded)
	decodedEvent, err := url.QueryUnescape(eventParam)
	if err != nil {
		log.Printf("⚠️ Failed to URL decode event parameter, using raw value: %v", err)
		decodedEvent = eventParam
	}

	log.Printf("📥 Amber event parameter: %s", decodedEvent)

	// Extract public key from event parameter
	publicKey, err := auth.extractPublicKeyFromAmber(decodedEvent)
	if err != nil {
		log.Printf("❌ Failed to extract public key from amber response: %v", err)
		auth.renderAmberError(w, "Invalid response from Amber: "+err.Error())
		return
	}

	log.Printf("✅ Amber callback processed successfully: %s...", publicKey[:16])

	// Check whitelist access before creating session
	if err := auth.checkWhitelistAccess(r, publicKey); err != nil {
		log.Printf("❌ Whitelist check failed for Amber login: %v", err)
		if whitelistErr, ok := err.(*WhitelistError); ok {
			auth.renderAmberWhitelistError(w, whitelistErr)
		} else {
			auth.renderAmberError(w, "Access denied: "+err.Error())
		}
		return
	}

	// Create session
	sessionRequest := session.SessionInitRequest{
		PublicKey:     publicKey,
		RequestedMode: session.WriteMode, // Game requires write mode
		SigningMethod: session.AmberSigning,
	}

	_, err = session.CreateUserSession(w, sessionRequest)
	if err != nil {
		log.Printf("❌ Failed to create amber session: %v", err)
		auth.renderAmberError(w, "Failed to create session")
		return
	}

	log.Printf("✅ Amber session created successfully: %s...", publicKey[:16])

	// Render success page with auto-redirect
	auth.renderAmberSuccess(w, publicKey)
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

// WhitelistError represents a whitelist access denial with form URL
type WhitelistError struct {
	Message string
	FormURL string
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
			FormURL: auth.config.Server.WhitelistFormURL,
		}
	}

	log.Printf("✅ Whitelisted pubkey allowed: %s...", pubkey[:16])
	return nil
}

func (auth *AuthHandler) renderAmberSuccess(w http.ResponseWriter, publicKey string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Amber Login Success - Pubkey Quest</title>
    <style>
        body {
            font-family: 'Pixelify Sans', monospace;
            margin: 0;
            padding: 20px;
            background: #001100;
            color: #00ff41;
            text-align: center;
        }
        .success { color: #00ff41; margin: 20px 0; }
        .loading { color: #888; }
    </style>
</head>
<body>
    <div class="success">
        <h2>✅ Amber Login Successful!</h2>
        <p>Connected successfully. Returning to Pubkey Quest...</p>
    </div>
    <div class="loading">
        <p>Please wait...</p>
    </div>

    <script>
        const amberResult = {
            success: true,
            publicKey: '` + publicKey + `',
            timestamp: Date.now()
        };

        try {
            localStorage.setItem('amber_callback_result', JSON.stringify(amberResult));
            console.log('Stored Amber success result in localStorage');
        } catch (error) {
            console.error('Failed to store Amber result:', error);
        }

        if (window.opener && !window.opener.closed) {
            try {
                window.opener.postMessage({
                    type: 'amber_success',
                    publicKey: '` + publicKey + `'
                }, window.location.origin);
                console.log('Sent success message to opener window');
            } catch (error) {
                console.error('Failed to send message to opener:', error);
            }
        }

        setTimeout(() => {
            try {
                if (window.opener && !window.opener.closed) {
                    window.close();
                } else {
                    window.location.href = '/game?amber_login=success';
                }
            } catch (error) {
                console.error('Failed to navigate:', error);
                window.location.href = '/game';
            }
        }, 1500);
    </script>
</body>
</html>`

	w.Write([]byte(html))
}

func (auth *AuthHandler) renderAmberError(w http.ResponseWriter, errorMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)

	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Amber Login Error - Pubkey Quest</title>
    <style>
        body {
            font-family: 'Pixelify Sans', monospace;
            margin: 0;
            padding: 20px;
            background: #001100;
            color: #ff4444;
            text-align: center;
        }
        .error { color: #ff4444; margin: 20px 0; }
        .retry { margin-top: 20px; }
        .retry a { color: #00ff41; text-decoration: none; }
    </style>
</head>
<body>
    <div class="error">
        <h2>❌ Amber Login Failed</h2>
        <p>` + errorMsg + `</p>
    </div>
    <div class="retry">
        <a href="/game">← Return to game</a>
    </div>

    <script>
        if (window.opener) {
            window.opener.postMessage({
                type: 'amber_error',
                error: '` + errorMsg + `'
            }, window.location.origin);
            setTimeout(() => window.close(), 3000);
        }
    </script>
</body>
</html>`

	w.Write([]byte(html))
}

func (auth *AuthHandler) renderAmberWhitelistError(w http.ResponseWriter, whitelistErr *WhitelistError) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)

	formLinkHTML := ""
	if whitelistErr.FormURL != "" {
		formLinkHTML = `<div class="form-link">
            <a href="` + whitelistErr.FormURL + `" target="_blank">Request Access →</a>
        </div>`
	}

	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Access Denied - Pubkey Quest</title>
    <style>
        body {
            font-family: 'Pixelify Sans', monospace;
            margin: 0;
            padding: 20px;
            background: #001100;
            color: #ff4444;
            text-align: center;
        }
        .error { color: #ff4444; margin: 20px 0; }
        .form-link { margin: 20px 0; }
        .form-link a {
            display: inline-block;
            padding: 10px 20px;
            background: #00ff41;
            color: #001100;
            text-decoration: none;
            border-radius: 4px;
            font-weight: bold;
        }
        .retry { margin-top: 20px; }
        .retry a { color: #888; text-decoration: none; }
    </style>
</head>
<body>
    <div class="error">
        <h2>🚫 Access Denied</h2>
        <p>` + whitelistErr.Message + `</p>
    </div>
    ` + formLinkHTML + `
    <div class="retry">
        <a href="/game">← Return to game</a>
    </div>

    <script>
        const whitelistDenialData = {
            type: 'whitelist_denial',
            error: '` + whitelistErr.Message + `',
            formUrl: '` + whitelistErr.FormURL + `'
        };

        // Store in localStorage as fallback
        try {
            localStorage.setItem('amber_callback_result', JSON.stringify(whitelistDenialData));
            console.log('Stored whitelist denial in localStorage');
        } catch (error) {
            console.error('Failed to store whitelist denial:', error);
        }

        // Send via postMessage
        if (window.opener && !window.opener.closed) {
            try {
                window.opener.postMessage(whitelistDenialData, window.location.origin);
                console.log('Sent whitelist denial via postMessage');
            } catch (error) {
                console.error('Failed to send postMessage:', error);
            }
        }

        // Auto-close after 5 seconds
        setTimeout(() => {
            try {
                if (window.opener && !window.opener.closed) {
                    window.close();
                } else {
                    window.location.href = '/game';
                }
            } catch (error) {
                console.error('Failed to close window:', error);
                window.location.href = '/game';
            }
        }, 5000);
    </script>
</body>
</html>`

	w.Write([]byte(html))
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
	response := map[string]interface{}{
		"success":          false,
		"error":            whitelistErr.Message,
		"whitelist_denial": true,
		"form_url":         whitelistErr.FormURL,
	}
	auth.sendJSONResponse(w, response, http.StatusForbidden)
}