# Quick Setup Guide

This tool uses **service accounts** for Google Sheets access - much simpler than OAuth2!

## 5-Minute Setup

### 1. Create Service Account (2 minutes)

1. Go to [console.cloud.google.com](https://console.cloud.google.com/)
2. Create a new project (or select existing)
3. Click **APIs & Services** → **Enable APIs and Services**
4. Search for "Google Sheets API" and **Enable** it
5. Go to **IAM & Admin** → **Service Accounts**
6. Click **Create Service Account**
   - Name: `anki-sync` (or anything you like)
   - Click **Create and Continue**
   - Skip optional steps, click **Done**
7. Click on your new service account
8. Go to **Keys** tab
9. Click **Add Key** → **Create New Key** → **JSON**
10. Save the downloaded file as `service-account.json`

### 2. Share Your Sheet (1 minute)

**CRITICAL STEP - Don't skip this!**

1. Open the JSON file you just downloaded
2. Find the `client_email` field - it looks like:
   ```
   your-service@project-id.iam.gserviceaccount.com
   ```
3. Open your Google Sheet
4. Click **Share** button (top right)
5. Paste the service account email
6. Give it **Editor** permissions
7. Click **Share**

### 3. Install the Tool (1 minute)

```bash
# Move the service account key to the right location
mkdir -p ~/.sync
mv ~/Downloads/service-account.json ~/.sync/

# Initialize configuration
./sync init
```

When prompted, enter:
- **Sheet ID**: Copy from your sheet's URL
  - URL: `https://docs.google.com/spreadsheets/d/YOUR_SHEET_ID/edit`
  - Just copy the `YOUR_SHEET_ID` part
- **Sheet Name**: Usually "Sheet1" (or whatever tab name you're using)
- **Anki Deck**: Name of your Anki deck (e.g., "Greek")

### 4. Test It (1 minute)

```bash
# Make sure Anki is running with AnkiConnect installed

# Test push (dry-run won't make changes)
./sync push --dry-run

# If that looks good, actually sync
./sync push
```

## Common Issues

### "Failed to read service account key file"

The file needs to be at `~/.sync/service-account.json`

```bash
# Check if it exists
ls ~/.sync/service-account.json

# If not, move it there
mv /path/to/your/downloaded-key.json ~/.sync/service-account.json
```

### "Permission denied" or "Failed to read sheet"

**You forgot to share the sheet with the service account!**

1. Open the `service-account.json` file
2. Copy the email from `client_email` field
3. Share your Google Sheet with that email (Editor permissions)

### "Cannot connect to AnkiConnect"

1. Make sure Anki is running
2. Go to **Tools** → **Add-ons**
3. If you don't see AnkiConnect, click **Get Add-ons**
4. Enter code: `2055492159`
5. Restart Anki

## What's Different from OAuth2?

**OAuth2** (complicated):
- Create OAuth2 credentials
- Configure consent screen
- Handle browser authorization
- Deal with token expiration
- Manage refresh tokens

**Service Account** (simple):
- Download one JSON file
- Share your sheet
- Done!

No browser popups, no authorization flows, perfect for automation.

## Need Help?

Check the main README.md for more details:
- Full command reference
- Google Sheet format
- Troubleshooting guide
- How sync works

## Security Note

The `service-account.json` file contains credentials. Keep it private!

It's already in `.gitignore` so won't be committed to Git, but make sure:
- Don't share it publicly
- Don't commit it to repositories
- Treat it like a password

The service account can ONLY access sheets you explicitly share with it, so it's quite safe for personal use.
