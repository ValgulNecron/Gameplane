# OIDC provider setup

Gameplane supports OpenID Connect (OIDC) for single sign-on. Providers are managed dynamically through the dashboard and applied without restart.

## Prerequisites

Before adding a provider, set **Admin Settings → General → External URL** to your public dashboard hostname (e.g. `https://gameplane.example.com`). The API refuses to build providers without it — the redirect/callback URL is derived from this setting.

## Adding a provider

**Admin Settings → Authentication → Add provider**

Fill in:
- **Issuer URL**: the identity provider's OpenID configuration endpoint
- **Client ID**: the public identifier from your IdP
- **Client Secret**: stored in a Kubernetes Secret (automatic)
- **Display name** (optional): login button text; defaults to the provider name

The callback URL is automatically set to `{External URL}/auth/oidc/{provider-name}/callback`.

**Note**: group→role mapping is only available on dashboard-registered providers — the legacy Helm-flag single OIDC provider (`--oidc-*` values) does not support it.

## Provider guides

### Keycloak

1. Log in to your Keycloak realm as an admin.

2. **Create a realm client**:
   - Clients → Create client
   - Client type: OpenID Connect
   - Client ID: (choose one, e.g. `gameplane`)
   - Next → Standard flow enabled → Save

3. **Configure the client**:
   - In the client's Access settings:
     - Root URL: leave blank (Keycloak doesn't enforce this for OIDC)
     - Valid Redirect URIs: add `https://gameplane.example.com/auth/oidc/keycloak/callback`
     - Web Origins: add `https://gameplane.example.com`
     - Save

4. **Get credentials**:
   - Credentials tab → copy the Client Secret

5. **Find the issuer URL**:
   - Realm settings → Endpoints → OpenID Endpoint Configuration
   - Copy the URL without the `/.well-known/openid-configuration` suffix
   - Example: `https://keycloak.example.com/realms/master`

6. **Expose groups** (optional, for role mapping from v0.2.0-beta.6+):
   - Client scopes → Add builtin mapper → Group Membership
   - Token Mapper Type: Group Membership
   - Full group path: OFF (so claims use short group names, not `/root/group`)
   - Add to ID Token: ON

7. **In Gameplane**, set:
   - Issuer: `https://keycloak.example.com/realms/master` (from step 5)
   - Client ID: `gameplane`
   - Client Secret: (from step 4)

### Authentik

1. Log in to your Authentik instance as an admin.

2. **Create an OAuth2/OpenID provider**:
   - Applications → Providers → Create → OpenID Provider / Generic OAuth
   - Name: `gameplane` (or your choice)

3. **Configure the provider**:
   - Authorization flow: select a default OAuth2 authorization flow (usually the built-in one)
   - Save — Authentik generates a Client ID and Client Secret

4. **Get credentials**:
   - Copy the Client ID and Client Secret from the provider page

5. **Find the issuer URL**:
   - Settings → System → Tenant settings
   - Copy the Tenant Domain (e.g. `https://authentik.example.com`)
   - Issuer URL is: `https://authentik.example.com/application/o/<provider-slug>/`
   - The slug is shown on the provider's overview page

6. **Create an application** (binds the provider to your dashboard):
   - Applications → Applications → Create
   - Name: `gameplane`
   - Provider: select the provider you just created
   - Redirect URIs: add `https://gameplane.example.com/auth/oidc/authentik/callback`
   - Save

7. **Groups are included by default**:
   - Authentik's OIDC profile scope automatically returns `ak_groups` in the ID token
   - From v0.2.0-beta.6+, these can be mapped to Gameplane roles via Authentication settings

8. **In Gameplane**, set:
   - Issuer: `https://authentik.example.com/application/o/<slug>/`
   - Client ID: (from step 4)
   - Client Secret: (from step 4)

### Google

1. Go to [Google Cloud Console](https://console.cloud.google.com).

2. **Create an OAuth client**:
   - Select your project (create one if needed)
   - APIs & Services → Credentials → Create Credentials → OAuth client ID
   - Application type: Web application

3. **Configure the client**:
   - Authorized JavaScript origins: add `https://gameplane.example.com`
   - Authorized redirect URIs: add `https://gameplane.example.com/auth/oidc/google/callback`
   - Create

4. **Get credentials**:
   - Copy the Client ID and Client Secret

5. **In Gameplane**, set:
   - Issuer: `https://accounts.google.com`
   - Client ID: (from step 4)
   - Client Secret: (from step 4)

**Note**: Google does not issue a `groups` claim for free tier accounts. If you need group-based role mapping, you must set up [Workspace Directory API](https://developers.google.com/workspace/directory/v1) and use a custom identity provider configuration (not a standard OIDC setup). For most users, simply promoting new logins to the operator or admin role covers the use case.

## Troubleshooting

| Issue | Check |
|-------|-------|
| "issuer discovery failed" (login page won't load) | **Wrong issuer URL or TLS failure.** Verify the exact issuer URL in your IdP's OIDC endpoints, including trailing slash if present. Test locally: `curl https://issuer/.well-known/openid-configuration`. If the IdP uses a private CA, the API container must trust it — there is no chart value for this today, so either use a publicly-trusted certificate on the IdP or mount your CA into the API pod's system trust store. |
| "redirect_uri mismatch" at login | **External URL changed after registering the app.** If you set a new `External URL` in Admin Settings, update the redirect URI in your IdP's client config to match `{new-url}/auth/oidc/{provider-name}/callback`. |
| "state mismatch" (blank login error) | **Cookies blocked or clock skew.** Ensure your browser accepts session cookies (`gameplane_oidc_state`, `gameplane_oidc_nonce`) and that server and IdP clocks are synchronized (within 5 minutes). Check browser DevTools → Network/Cookies for blocked third-party cookies if in an iframe. |
| Login succeeds but user is stuck at viewer role | **New OIDC users start as viewer.** An admin must promote them under Admin Settings → Users. From v0.2.0-beta.6+, group claims can map to roles automatically; check Authentication settings for group-based role mapping. |
| Provider button missing from login page | **Provider disabled or secret missing.** Verify in Admin Settings → Authentication that the provider is enabled and has a client secret stored. If the secret was deleted from the control-plane namespace (label `gameplane.local/auth-provider=true`), recreate it with the exact name shown in the provider's config. |
