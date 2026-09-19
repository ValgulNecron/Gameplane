# Contract: User Preferences API

**Feature**: `016-user-theme-customization`  
**Binding Modules**: `api/internal/handlers/users.go`, `api/internal/db/`, `web/src/lib/endpoints.ts`  
**Status**: Binding  

---

## 1. Endpoints

### 1.1 `GET /api/v1/users/me/preferences`

Retrieves the authenticated user's current styling and theme preferences.

- **Authentication**: Required (valid session cookie).
- **Permissions**: None (any authenticated user can read their own preferences).

#### Response: `200 OK`

```json
{
  "themeType": "preset",
  "presetId": "legacy",
  "appearanceMode": "system",
  "customColors": {
    "accent": "#3B82F6",
    "surface": "#18181B"
  },
  "customCss": "/* optional user styles */",
  "updatedAt": "2026-09-19T14:32:00Z"
}
```

*Note: If no record exists in `user_preferences` for this user (e.g. newly created user), the API returns the default configuration:*

```json
{
  "themeType": "preset",
  "presetId": "pink",
  "appearanceMode": "system",
  "customColors": null,
  "customCss": null,
  "updatedAt": "2026-09-19T14:32:00Z"
}
```

#### Error Responses

- `401 Unauthorized`: When session cookie is missing or invalid.

---

### 1.2 `PUT /api/v1/users/me/preferences`

Updates the authenticated user's styling preferences.

- **Authentication**: Required (valid session cookie + CSRF header).
- **Permissions**: None (users manage their own styling preferences).

#### Request Body

```json
{
  "themeType": "custom_colors",
  "presetId": "pink",
  "appearanceMode": "dark",
  "customColors": {
    "accent": "#10B981",
    "surface": "#121114"
  },
  "customCss": ".dashboard-card { border-radius: 12px; }"
}
```

#### Validation Rules

- `themeType`: Required. Must be one of `"preset"`, `"custom_colors"`, `"custom_css"`.
- `presetId`: Required. Must be one of `"pink"`, `"legacy"`.
- `appearanceMode`: Required. Must be one of `"light"`, `"dark"`, `"system"`.
- `customColors`: Optional object. If provided, `accent` and `surface` must match `^#([0-9a-fA-F]{6})$`.
- `customCss`: Optional string. Max length 65,536 bytes. Must not contain `<style`, `</style`, `<script`, or `</script`.

#### Response: `200 OK`

Returns the updated `UserThemePreferences` object.

#### Error Responses

- `400 Bad Request`: Validation failure (invalid color format, invalid enum, forbidden tag in CSS, or payload exceeding size limit).
  ```json
  { "error": "customCss contains forbidden HTML tag" }
  ```
- `401 Unauthorized`: Missing or invalid authentication session.
- `403 Forbidden`: CSRF token mismatch or expired.

---

### 1.3 Extension to `GET /api/v1/users/me`

To avoid an extra network round-trip on dashboard boot, `GET /api/v1/users/me` is extended to include the preferences object directly in the response payload:

```json
{
  "id": 42,
  "username": "alex",
  "displayName": "Alex Rivera",
  "email": "alex@example.com",
  "role": "operator",
  "provider": "local",
  "createdAt": "2026-08-15T10:00:00Z",
  "permissions": { ... },
  "preferences": {
    "themeType": "preset",
    "presetId": "legacy",
    "appearanceMode": "system",
    "customColors": null,
    "customCss": null,
    "updatedAt": "2026-09-19T14:32:00Z"
  }
}
```
