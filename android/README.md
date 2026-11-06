# Cookira for Android

Native Android client for a self-hosted [Cookira](../README.md) server, built with
**Kotlin** and **Jetpack Compose** (same toolchain as Domitara's Android app). It talks to
the server's REST API (`/api/v1`) with the token mode the server already offers for
non-browser clients: no backend changes needed.

## Features (parity with the web UI)

- **Connect / sign in** to any Cookira server by address; first-run **admin setup**;
  **accept an invite** by pasting or sharing the invite link into the app.
- **Recipes**: search, tag filter, All/Published/Drafts, infinite scroll, pull to refresh.
- **Recipe view**: photo, stats, tick-off ingredients, lettered sub-steps, markdown notes,
  keep-screen-on while cooking (toggle in Settings), share as plain text (replaces Print).
- **Create / edit**: all fields, ingredient groups and steps with reorder, tags with
  suggestions, status, **photo** from gallery or camera (downscaled before upload).
- **Offline reading**: the whole library is saved on the phone (synced on sign-in, on pull to
  refresh and when it goes stale) and recipe photos you view are kept, so list, search, tags
  and the cooking view work with no connection. A banner shows when you're looking at the saved
  copy and how old it is; editing, publishing, sharing and deleting need a connection. Settings →
  Offline library has Sync now, Download all photos and Clear. The copy is wiped on sign-out.
- **Publish / unpublish**, **delete**, **history with restore**.
- **Share links**: create (with lifetime), copy/send, revoke; **open a share link** (paste
  it on the connect screen, or share it into the app) for the public read-only view.
- **Settings**: change name/password, theme, **household** admin (invite links, add with
  password, roles, reset password, remove, pending invites), **connected apps** (MCP/OAuth),
  how to connect an agent.

Not in the app: the OAuth consent page (`/oauth/authorize`), which only agents use.

## Build

```bash
cd android
./gradlew assembleDebug            # installs side by side as com.deranjer.cookira.debug
./gradlew lintDebug testDebugUnitTest
```

Needs JDK 17+ and the Android SDK with platform 37 (`ANDROID_HOME` or `local.properties`).
In the emulator the host's server is `http://10.0.2.2:8080`.

## Releases and the F-Droid repo

Same flow as Domitara and BookOrbit: a signed APK is attached to a GitHub Release, and the
[F-Droid repo](https://deranjer.github.io/fdroid/repo) (`deranjer/fdroid`) downloads the
latest one daily and re-signs the index.

1. **Automatic:** every Android change that lands on `main` and passes `android-checks.yml`
   is tagged with the next patch version (`android-auto-release.yml`: v0.1.0 first, then
   v0.1.1, ...) and `android-release.yml` builds `cookira-X.Y.Z.apk` and creates the GitHub
   Release. `versionCode = major*10000 + minor*100 + patch`.
   **Manual:** for a minor or major bump, push the tag yourself, e.g.
   `git tag v0.2.0 && git push origin v0.2.0`; `ci.yml` also publishes the Docker image for
   that tag, and automatic releases continue from it. Tags with a `-suffix` are prereleases and
   are skipped by the F-Droid repo.
2. The `Update F-Droid repo` workflow in `deranjer/fdroid` (daily, or run it by hand) picks it up.

### One-time setup

Create a release keystore for the app (keep it; changing it later breaks updates) and add
these **secrets to this repo** (`deranjer/cookira`):

```bash
keytool -genkeypair -v -keystore cookira-release.jks -alias cookira -keyalg RSA -keysize 4096 -validity 10000
base64 -w0 cookira-release.jks | gh secret set ANDROID_KEYSTORE_BASE64 --repo deranjer/cookira
gh secret set ANDROID_KEYSTORE_PASSWORD --repo deranjer/cookira
gh secret set ANDROID_KEY_ALIAS --repo deranjer/cookira      # cookira
gh secret set ANDROID_KEY_PASSWORD --repo deranjer/cookira
```

In `deranjer/fdroid`: merge `metadata/com.deranjer.cookira.yml` and the "Download Cookira
APK" step. If this repo is private, also set `COOKIRA_RELEASE_TOKEN` there.

## Layout

```
app/src/main/java/com/deranjer/cookira/
  data/    Retrofit API + DTOs, token refresh (OkHttp authenticator), DataStore session, repositories
  di/      AppContainer (manual DI, no Hilt/KSP)
  ui/      Compose screens + ViewModels (auth, recipes, settings), theme, navigation
```
