# Troubleshooting Audio Issues

## Issue: Audio not showing up on Anki cards

### Step 1: Verify TTS is enabled in config

Check your `~/.sync/config.yaml` file:

```yaml
anki_profile: "User 1"  # Your Anki profile name (must match exactly)

text_to_speech:
  enabled: true  # Must be true
  voice_name: "el-GR-Wavenet-A"
  audio_encoding: "MP3"
  speaking_rate: 1.0
  pitch: 0.0
  volume_gain_db: 0.0
  request_delay_ms: 100
```

If the `text_to_speech` section is missing or `enabled: false`, audio won't be generated.

**Important:** The `anki_profile` setting must match your Anki profile name exactly. Check in Anki: File → Switch Profile to see available profile names. The default is "User 1".

### Step 2: Run push with verbose logging

```bash
./sync push --verbose
```

Look for these log messages:

#### ✅ Good signs:

**New cards with fresh audio:**
```
TTS client initialized successfully
Generated audio for 'γεια' (XXXX bytes)
Uploading and attaching audio 'γεια.mp3' to card 'hello'
Created card 'hello' with Anki ID 123456789
```

**New cards with existing audio:**
```
Audio already exists for 'γεια', linking to card
Linking existing audio 'γεια.mp3' to card 'hello'
Created card 'hello' with Anki ID 123456789
```

**Updated cards with new/changed Greek text:**
```
Generated audio for 'νέο' (XXXX bytes)
Uploading updated audio 'νέο.mp3' for card 'new'
Updated card 'new' (Anki ID 123456789)
```

**Updated cards with existing audio:**
```
Audio already exists for 'νέο', linking to card
Linking audio 'νέο.mp3' to updated card 'new'
Updated card 'new' (Anki ID 123456789)
```

#### ❌ Problem indicators:
```
TTS is disabled, skipping audio generation
# → Fix: Enable TTS in config

ERROR: Failed to initialize TTS client: ...
# → Fix: Check service account permissions

WARNING: Skipping audio generation for card 'X': Greek text is empty
# → Fix: Ensure Greek column has text

ERROR: Failed to generate audio for 'X': ...
# → Fix: Check TTS API quota and permissions
```

### Step 3: Check service account permissions

Your service account needs the Text-to-Speech API enabled:

1. Go to https://console.cloud.google.com/
2. Select your project
3. Go to "APIs & Services" → "Library"
4. Search for "Cloud Text-to-Speech API"
5. Click "ENABLE" if not already enabled
6. Go to "IAM & Admin" → "IAM"
7. Find your service account
8. Ensure it has role: `Cloud Text-to-Speech User` (or `roles/cloudtts.user`)

### Step 4: Verify audio files in Anki

The tool checks for existing audio files in your Anki media directory before generating:
- **macOS**: `~/Library/Application Support/Anki2/{profile}/collection.media`
- **Windows**: `%APPDATA%/Anki2/{profile}/collection.media`
- **Linux**: `~/.local/share/Anki2/{profile}/collection.media`

To verify:
1. Open Anki
2. Go to Tools → Check Media
3. Look for `.mp3` files with Greek names (e.g., `γεια.mp3`)
4. If you see "X files in media folder but not used by any cards", the files exist but aren't attached

**Tip:** You can also check the directory directly to see if audio files exist.

### Step 5: Check the card content in Anki

1. Open the card in Anki's browser (Browse → select your deck)
2. Look at the "Back" field
3. You should see something like: `γεια[sound:γεια.mp3]`
4. If you only see `γεια` without `[sound:...]`, the audio wasn't attached

### Step 6: Test audio manually

1. Open Anki
2. Browse cards
3. Select a card
4. Look at the Back field - should contain `[sound:filename.mp3]`
5. Click the speaker icon in the preview pane to test audio

### Step 7: Create a test card

Try creating a single test card:

```bash
# Add a row to your Google Sheet:
# | Anki ID | Checksum | English | Greek | Part of Speech |
# |         |          | test    | τεστ  | Noun           |

./sync push --verbose
```

Watch the output carefully for any errors.

### Step 8: Check AnkiConnect version

1. Open Anki
2. Go to Tools → Add-ons
3. Select AnkiConnect
4. Click "View Files"
5. Open `manifest.json`
6. Check the version - should be recent (2.0.13+)

If older, update:
1. Tools → Add-ons → Get Add-ons
2. Enter code: `2055492159`
3. Restart Anki

### Common Issues

#### Issue: "TTS is disabled"
**Solution:** Add the `text_to_speech` section to `~/.sync/config.yaml` with `enabled: true`

#### Issue: "Failed to initialize TTS client"
**Solution:**
- Verify Text-to-Speech API is enabled in Google Cloud Console
- Check service account has `roles/cloudtts.user` role
- Verify service account JSON key file is valid

#### Issue: Audio generates but doesn't play
**Solution:**
- Check Anki's audio settings: Tools → Preferences → Review → "Replay audio when answer shown"
- Test if Anki can play audio at all (try a different card with audio)
- Check your system audio settings

#### Issue: Greek characters in filename causing issues
**Solution:** This shouldn't be an issue, but if it is:
- Check `Tools → Check Media` in Anki
- Look for error messages about invalid filenames

### Debug Mode

For maximum verbosity:

```bash
./sync push --verbose --debug
```

This will show:
- Every API call to AnkiConnect
- Full card data being sent
- Detailed error messages

### Still not working?

If audio still doesn't appear:

1. Check if `~/.sync/service-account.json` exists and is valid
2. Try creating a card manually in Anki with a `[sound:test.mp3]` tag to verify Anki's audio works
3. Check Anki's error console: Help → Debug Console → Check for errors
4. Verify the card's note type is "VocabSync" (should be auto-created)

### Manual Test

Create a card manually in Anki:
1. Add → VocabSync note type
2. Front: "test"
3. Back: "τεστ[sound:test.mp3]"
4. Click the speaker icon to see if Anki can find/play the audio

If this doesn't work, the issue is with Anki's audio system, not the sync tool.
