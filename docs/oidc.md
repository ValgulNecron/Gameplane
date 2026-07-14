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
| Login succeeds but role did not update when groups changed at IdP | **Last-admin demotion guard.** When group→role mapping is enabled, users' roles are re-synced from their IdP groups on each login. To prevent lockout, the system blocks any role change that would remove the *last* user capable of managing other users (i.e., the last admin or user-manager). If your group assignment change would demote the sole user-manager, the demotion is silently skipped: the login succeeds, but the user's stored role remains unchanged. To confirm the demotion guard was triggered, check the API pod logs for a warn-level entry containing `oidc role resync skipped`. Verify you have at least one other admin or user-manager in a different group, then have the affected user log in again. |
| User created in one IdP logs in as a different account after linking IdP accounts | **Account linking is by OIDC identity, not email.** OIDC identities are permanently linked to Gameplane accounts by (issuer, subject) — the combination of your identity provider and the user's subject claim (a unique per-provider identifier, not email). If a user's email address or display name changes at the IdP, they will still log in as the same Gameplane account. However, if you add a different IdP or reconfigure an existing one to use a different subject attribute, that user will be treated as a new person on first login with the new issuer/subject pair. To prevent duplicate accounts, ensure your IdP subject claim is stable and won't change if you reconfigure the provider. |
| Provider button missing from login page | **Provider disabled or secret missing.** Verify in Admin Settings → Authentication that the provider is enabled and has a client secret stored. If the secret was deleted from the control-plane namespace (label `gameplane.local/auth-provider=true`), recreate it with the exact name shown in the provider's config. |
