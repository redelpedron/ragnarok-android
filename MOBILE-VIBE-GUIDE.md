# Mobile Vibe Coding Guide

You are reading this on a phone. Good — you don't need Android Studio,
a PC, or even the `go` command installed locally. Everything happens in
GitHub Actions.

## Step-by-step (do this on your phone)

### 1. Create your repo

1. Open **GitHub** in your mobile browser.
2. Tap **+ → New repository**.
3. Name it `goro-android` (or whatever).
4. Make it **Public** (Actions are free and unlimited for public repos).
5. Tap **Create repository**.

### 2. Upload the scaffold files

You need to get the files from this scaffold into your repo.

**Option A: Fork (easiest)**
- If someone already uploaded this scaffold as a template repo, tap **Fork**.

**Option B: Upload ZIP**
1. Download `goro-android-port.zip` from the release / conversation.
2. On your phone, extract it (Files app, ZArchiver, etc.).
3. Open GitHub mobile → your repo → **Add file → Upload files**.
4. Select all extracted files. Tap **Commit**.

**Option C: GitHub Web Editor (no ZIP needed)**
1. In your repo, tap `.` (period) on your phone keyboard — this opens the
   GitHub web editor.
2. Create each file manually by path:
   - `.github/workflows/build-android.yml`
   - `android/app/build.gradle`
   - `android/app/src/main/AndroidManifest.xml`
   - etc.
3. Paste the contents from this scaffold.

### 3. Trigger your first build

1. In your repo, tap **Actions**.
2. You should see the **Build Android APK** workflow.
3. Tap it, then tap **Run workflow → Run workflow**.
4. Wait ~5–8 minutes. The runner downloads the NDK, clones goro, applies
   patches, and builds the APK.

### 4. Download the APK

1. When the workflow finishes (green checkmark), tap into the run.
2. Scroll to **Artifacts**.
3. Tap **goro-android-debug** to download `app-debug.apk`.
4. Transfer it to your phone (if you downloaded on another device) and install.

   **Android 8+:** You may need to allow "Install unknown apps" for your
   browser/files app.

### 5. Provide game data

The APK does **not** contain RO assets (copyright). On first launch:

1. The app asks you to pick a folder.
2. Select the folder containing your `data.grf` (and `rdata.grf` if renewal).

**Where to get data.grf:**
- Copy it from an existing PC RO client installation.
- Or download a pre-renewal repack (~775 MB) and extract it on your phone.
- Place it in `/sdcard/Android/data/com.goro.android/files/` to skip the picker.

### 6. Release builds (optional)

Instead of downloading artifacts, you can publish a proper GitHub Release:

1. Go to **Actions → Build Android APK**.
2. Tap **Run workflow**.
3. Toggle **Create GitHub Release?** to `true`.
4. The workflow will create a signed APK and attach it to a release tag.
5. Go to **Releases** in your repo to download it.

## Troubleshooting from mobile

| Problem | Fix |
|---|---|
| **Workflow fails at "Apply patches"** | Upstream goro/gogpu may have changed. Edit the patch files in the web editor to match the current source. |
| **APK installs but crashes** | Check **Actions → Build logs → Build Go shared library** for compile errors. Also verify your phone supports Vulkan. |
| **Can't upload files on mobile** | Use the GitHub app → your repo → **Browse** → tap the `...` menu → **Add file**. Or use a Git client like MGit or Termux. |
| **GRF picker never shows** | Grant **Files and media** permission to the app in Android Settings. |

## What the CI does (summary)

Every time you push or tap "Run workflow":

1. Spins up an Ubuntu runner in the cloud.
2. Installs Go 1.25, Android NDK r27, and Android SDK build-tools.
3. Clones `kivutar/goro` and `gogpu/gogpu`.
4. Automatically applies the Android patches from your repo.
5. Cross-compiles `libgoro.so` for `android/arm64`.
6. Packages it into an APK with Gradle.
7. Uploads the APK as a downloadable artifact (or Release).

You write code on your phone. GitHub builds it in the cloud. You install the APK
on your phone. Full loop, zero PC.
