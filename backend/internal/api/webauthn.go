package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"ticopay/backend/internal/config"
)

// newWebAuthn derives the Relying Party config from the configured CORS origin
// (RP ID = its hostname, RP origin = the full URL), so no extra env vars are
// needed. Returns nil if the origin can't be parsed.
func newWebAuthn(cfg config.Config) *webauthn.WebAuthn {
	if len(cfg.CORSOrigins) == 0 {
		return nil
	}
	u, err := url.Parse(cfg.CORSOrigins[0])
	if err != nil || u.Hostname() == "" {
		return nil
	}
	w, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "Tico Pay",
		RPID:          u.Hostname(),
		RPOrigins:     cfg.CORSOrigins,
	})
	if err != nil {
		return nil
	}
	return w
}

// --- webauthn.User implementation ---

type waUser struct {
	id      []byte
	name    string
	display string
	creds   []webauthn.Credential
}

func (u *waUser) WebAuthnID() []byte                         { return u.id }
func (u *waUser) WebAuthnName() string                       { return u.name }
func (u *waUser) WebAuthnDisplayName() string                { return u.display }
func (u *waUser) WebAuthnIcon() string                       { return "" }
func (u *waUser) WebAuthnCredentials() []webauthn.Credential { return u.creds }

func (a *App) loadCredentials(ctx context.Context, uid string) ([]webauthn.Credential, error) {
	rows, err := a.pool.Query(ctx,
		`SELECT id, public_key, attestation_type, aaguid, sign_count, transports, backup_eligible, backup_state
		 FROM webauthn_credentials WHERE user_id = $1`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	creds := make([]webauthn.Credential, 0)
	for rows.Next() {
		var id, pk, aaguid []byte
		var attType, transports string
		var signCount int64
		var backupEligible, backupState bool
		if err := rows.Scan(&id, &pk, &attType, &aaguid, &signCount, &transports, &backupEligible, &backupState); err != nil {
			return nil, err
		}
		creds = append(creds, webauthn.Credential{
			ID:              id,
			PublicKey:       pk,
			AttestationType: attType,
			Transport:       parseTransports(transports),
			Flags: webauthn.CredentialFlags{
				UserPresent:    true,
				BackupEligible: backupEligible,
				BackupState:    backupState,
			},
			Authenticator: webauthn.Authenticator{
				AAGUID:    aaguid,
				SignCount: uint32(signCount),
			},
		})
	}
	return creds, rows.Err()
}

func (a *App) loadWAUserByID(ctx context.Context, uid string) (*waUser, error) {
	var email, name string
	if err := a.pool.QueryRow(ctx, `SELECT email, full_name FROM users WHERE id = $1`, uid).Scan(&email, &name); err != nil {
		return nil, err
	}
	uu, err := uuid.Parse(uid)
	if err != nil {
		return nil, err
	}
	creds, err := a.loadCredentials(ctx, uid)
	if err != nil {
		return nil, err
	}
	return &waUser{id: uu[:], name: email, display: name, creds: creds}, nil
}

func (a *App) loadWAUserByEmail(ctx context.Context, email string) (*waUser, string, error) {
	var uid, name string
	if err := a.pool.QueryRow(ctx, `SELECT id, full_name FROM users WHERE email = $1`, email).Scan(&uid, &name); err != nil {
		return nil, "", err
	}
	uu, err := uuid.Parse(uid)
	if err != nil {
		return nil, "", err
	}
	creds, err := a.loadCredentials(ctx, uid)
	if err != nil {
		return nil, "", err
	}
	return &waUser{id: uu[:], name: email, display: name, creds: creds}, uid, nil
}

func parseTransports(s string) []protocol.AuthenticatorTransport {
	if s == "" {
		return nil
	}
	out := []protocol.AuthenticatorTransport{}
	for _, p := range strings.Split(s, ",") {
		if p != "" {
			out = append(out, protocol.AuthenticatorTransport(p))
		}
	}
	return out
}

func joinTransports(t []protocol.AuthenticatorTransport) string {
	parts := make([]string, len(t))
	for i, x := range t {
		parts[i] = string(x)
	}
	return strings.Join(parts, ",")
}

// --- short-lived signed session (challenge) round-trip ---

func (a *App) waSign(sd *webauthn.SessionData) (string, error) {
	b, err := json.Marshal(sd)
	if err != nil {
		return "", err
	}
	claims := jwt.MapClaims{
		"sd":  base64.RawStdEncoding.EncodeToString(b),
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(a.cfg.JWTSecret))
}

func (a *App) waParse(token string) (*webauthn.SessionData, error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		return []byte(a.cfg.JWTSecret), nil
	})
	if err != nil || !parsed.Valid {
		return nil, errors.New("invalid session")
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid session")
	}
	s, ok := claims["sd"].(string)
	if !ok {
		return nil, errors.New("invalid session")
	}
	b, err := base64.RawStdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	var sd webauthn.SessionData
	if err := json.Unmarshal(b, &sd); err != nil {
		return nil, err
	}
	return &sd, nil
}

// waErrDetail surfaces the underlying WebAuthn error (incl. go-webauthn's
// DevInfo) for debugging. TODO: revert to generic messages once stable.
func waErrDetail(err error) string {
	var pe *protocol.Error
	if errors.As(err, &pe) {
		return pe.Type + " | " + pe.Details + " | " + pe.DevInfo
	}
	return err.Error()
}

// --- registration (enroll a passkey while logged in) ---

func (a *App) handlePasskeyRegisterBegin(w http.ResponseWriter, r *http.Request) {
	if a.wa == nil {
		writeError(w, http.StatusServiceUnavailable, "passkeys no disponibles")
		return
	}
	user, err := a.loadWAUserByID(r.Context(), userID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo cargar el usuario")
		return
	}

	exclusions := make([]protocol.CredentialDescriptor, 0, len(user.creds))
	for _, c := range user.creds {
		exclusions = append(exclusions, c.Descriptor())
	}
	authSelect := protocol.AuthenticatorSelection{
		ResidentKey:      protocol.ResidentKeyRequirementRequired,
		UserVerification: protocol.VerificationPreferred,
	}

	options, session, err := a.wa.BeginRegistration(user,
		webauthn.WithAuthenticatorSelection(authSelect),
		webauthn.WithExclusions(exclusions),
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo iniciar el registro de la llave")
		return
	}
	token, err := a.waSign(session)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo iniciar el registro")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"publicKey": options.Response, "sessionToken": token})
}

func (a *App) handlePasskeyRegisterFinish(w http.ResponseWriter, r *http.Request) {
	if a.wa == nil {
		writeError(w, http.StatusServiceUnavailable, "passkeys no disponibles")
		return
	}
	var req struct {
		SessionToken string          `json:"sessionToken"`
		Credential   json.RawMessage `json:"credential"`
		Name         string          `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	session, err := a.waParse(req.SessionToken)
	if err != nil {
		writeError(w, http.StatusBadRequest, "sesión inválida o expirada")
		return
	}
	user, err := a.loadWAUserByID(r.Context(), userID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo cargar el usuario")
		return
	}
	parsed, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(req.Credential))
	if err != nil {
		writeError(w, http.StatusBadRequest, "credencial inválida: "+waErrDetail(err))
		return
	}
	credential, err := a.wa.CreateCredential(user, *session, parsed)
	if err != nil {
		writeError(w, http.StatusBadRequest, "no se pudo registrar la llave: "+waErrDetail(err))
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "Llave de acceso"
	}
	if _, err := a.pool.Exec(r.Context(),
		`INSERT INTO webauthn_credentials
		   (id, user_id, public_key, attestation_type, aaguid, sign_count, transports, name, backup_eligible, backup_state)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		credential.ID, userID(r), credential.PublicKey, credential.AttestationType,
		credential.Authenticator.AAGUID, int64(credential.Authenticator.SignCount),
		joinTransports(credential.Transport), name,
		credential.Flags.BackupEligible, credential.Flags.BackupState,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo guardar la llave")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

// --- passwordless login with a passkey ---

func (a *App) handlePasskeyLoginBegin(w http.ResponseWriter, r *http.Request) {
	if a.wa == nil {
		writeError(w, http.StatusServiceUnavailable, "passkeys no disponibles")
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))

	user, _, err := a.loadWAUserByEmail(r.Context(), email)
	if err != nil {
		writeError(w, http.StatusNotFound, "no encontramos esa cuenta")
		return
	}
	if len(user.creds) == 0 {
		writeError(w, http.StatusBadRequest, "esta cuenta no tiene llave de acceso")
		return
	}

	options, session, err := a.wa.BeginLogin(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo iniciar el login")
		return
	}
	token, err := a.waSign(session)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo iniciar el login")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"publicKey": options.Response, "sessionToken": token})
}

func (a *App) handlePasskeyLoginFinish(w http.ResponseWriter, r *http.Request) {
	if a.wa == nil {
		writeError(w, http.StatusServiceUnavailable, "passkeys no disponibles")
		return
	}
	var req struct {
		SessionToken string          `json:"sessionToken"`
		Credential   json.RawMessage `json:"credential"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	session, err := a.waParse(req.SessionToken)
	if err != nil {
		writeError(w, http.StatusBadRequest, "sesión inválida o expirada")
		return
	}
	uu, err := uuid.FromBytes(session.UserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "sesión inválida")
		return
	}
	uid := uu.String()
	user, err := a.loadWAUserByID(r.Context(), uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo cargar el usuario")
		return
	}
	parsed, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(req.Credential))
	if err != nil {
		writeError(w, http.StatusBadRequest, "credencial inválida: "+waErrDetail(err))
		return
	}
	credential, err := a.wa.ValidateLogin(user, *session, parsed)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "no se pudo verificar: "+waErrDetail(err))
		return
	}

	// Update the signature counter (clone detection).
	_, _ = a.pool.Exec(r.Context(),
		`UPDATE webauthn_credentials SET sign_count = $1 WHERE id = $2`,
		int64(credential.Authenticator.SignCount), credential.ID)

	u, err := a.fetchUser(r.Context(), uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo cargar el usuario")
		return
	}
	accounts, err := a.fetchAccounts(r.Context(), uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo cargar las cuentas")
		return
	}
	a.issueAuthResponse(w, http.StatusOK, u, accounts)
}

// --- management ---

func (a *App) handleListPasskeys(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(),
		`SELECT id, name, created_at FROM webauthn_credentials WHERE user_id = $1 ORDER BY created_at`, userID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudieron cargar las llaves")
		return
	}
	defer rows.Close()

	type pk struct {
		ID        string    `json:"id"`
		Name      string    `json:"name"`
		CreatedAt time.Time `json:"createdAt"`
	}
	list := make([]pk, 0)
	for rows.Next() {
		var id []byte
		var name string
		var created time.Time
		if err := rows.Scan(&id, &name, &created); err != nil {
			writeError(w, http.StatusInternalServerError, "no se pudieron leer las llaves")
			return
		}
		list = append(list, pk{ID: base64.RawURLEncoding.EncodeToString(id), Name: name, CreatedAt: created})
	}
	writeJSON(w, http.StatusOK, map[string]any{"passkeys": list})
}

func (a *App) handleDeletePasskey(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := base64.RawURLEncoding.DecodeString(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	if _, err := a.pool.Exec(r.Context(),
		`DELETE FROM webauthn_credentials WHERE id = $1 AND user_id = $2`, id, userID(r)); err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo eliminar la llave")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
