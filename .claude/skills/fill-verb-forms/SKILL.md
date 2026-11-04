---
name: fill-verb-forms
description: Fill in the Voice, Active Form, Future Simple, Aorist and Overrides columns (M–Q of the `words` tab) for Greek verbs in the vocabulary Google Sheet that feeds this Anki deck. Use this whenever the user wants verb forms, principal parts, conjugation columns or overrides added or updated for a Greek verb, mentions a sheet row that needs its verb columns filled, has just added new verbs to the sheet, or asks which verbs are missing forms, even if they don't name the columns.
---

# Fill verb forms

Each verb row in the `words` tab carries principal parts that end up on an Anki card. This skill works out the missing ones and writes them into the sheet. A wrong form gets memorized by the person studying the card, so accuracy matters more than filling every cell.

## 1. Find the sheet

Read `google_sheet_id` and `sheet_name` from `~/.config/sync/config.yaml`. This repo is public, so the ID stays in that file rather than in this skill.

Load the Google Sheets `get_values` and `update_values` tools. If they're missing, ask the user to enable the Google Sheets connector and stop.

Read `words!M1:R1`. Expect `voice`, `active form`, `future simple`, `aorist`, `overrides`, `dictionary`. If the columns have moved, stop and tell the user: everything below assumes these positions.

## 2. Choose the rows

- **The user names a row:** use it, and confirm column G is `verb`.
- **The user names a verb:** search column D for it. If it appears on more than one row, ask which one.
- **Otherwise:** read `words!D:D` and `words!G:Q`, then take the rows where G is `verb` and either M is blank, or O and P are both blank and Q doesn't contain `no perfective`. In both reads, array position + 1 is the row number. If nothing matches, say so and stop.

For each row read C–H and M–Q. Column C gives the meaning, which can decide between forms, and column H gives the conjugation group.

Leave a filled cell alone unless the user asks to change it, and flag anything in it that looks wrong instead of fixing it.

## 3. Work out the forms

Read `'verb tenses'!S2:W` for the perfective stem rules by group. They are the user's own rules, so use them rather than a different scheme. They don't settle every case. Present stems in `-ζ`, for example, split between `-σ-` (αγοράζω) and `-ξ-` (παίζω), so check what the verb actually does, and compare similar verbs already in the sheet.

If you can't confirm a form, leave its cell blank and say why in the report.

### M: voice

One of `active`, `passive` or `deponent`, in lowercase:

- `-ω`/`-ώ` is `active`.
- A `-μαι` verb is `passive` if it pairs with an active verb of related meaning (σηκώνομαι, γίνομαι).
- A `-μαι` verb with no such partner is `deponent` (έρχομαι, κοιμάμαι, είμαι).

### N: active form

Fill this only for `passive` verbs: the first-person present of the active verb it pairs with (σηκώνομαι → σηκώνω). Leave it blank for `active` and `deponent` verbs.

### O: future simple

First-person singular perfective, without θα: `γράψω`, `πιω`. Monosyllables take no accent: `δω`, `βρω`, `πω`. This can match the present form: κάνω → `κάνω`, σερβίρω → `σερβίρω`.

### P: aorist

First-person singular simple past, stressed on the third syllable from the end:

- Two-syllable forms take the augment `έ-`: `έγραψα`, `έφυγα`.
- Prepositional compounds take it inside the word: `επέστρεψα`, `απέτυχα`, `παρέλαβα`.

### Q: overrides

Leave this blank for a regular verb, which is the usual case. Add an entry only for a form that can't be predicted from O and P. Nothing in this repo parses the column, so the existing rows are the only spec. Copy their format exactly: lowercase keys, entries joined with `; `, in this order.

| Entry | Meaning | Example verb → value |
|---|---|---|
| `no perfective` | No perfective stem. Leave O and P blank. | είμαι → `no perfective` |
| `past: X` | Imperfect with an unexpected stem or augment | θέλω → `past: ήθελα`<br>επιστρέφω → `past: επέστρεφα` |
| `imp: sing, plural` | Irregular perfective imperative | έρχομαι → `imp: έλα, ελάτε` |
| `imp-cont: sing, plural` | Irregular continuous imperative | πάω → `imp-cont: πήγαινε, πηγαίνετε` |

Use `—` for a form that doesn't exist or isn't used: `imp: —, ιδωθείτε`, `imp: —, —`.

Combined example, έχω: `no perfective; past: είχα`.

Older rows don't list every irregular imperative (τρώω has none, for instance). Follow the format above, but don't go back and fill those in unless asked.

## 4. Write

Write each row with `update_values` to `words!M<row>:Q<row>`. Sheets may have been re-sorted since the row was read, so first re-read D for that row and confirm it's still the same verb.

Send `null` for every cell that should stay as it is, including blank N and Q and anything already filled. `null` skips a cell, while `""` clears it.

```json
{"range": "words!M943:Q943", "values": [["active", null, "κερδίσω", "κέρδισα", null]]}
```

Don't run `./sync push`. The user decides when to push to Anki.

## 5. Verify and report

Read `words!D<row>:Q<row>` back and check the values landed on the right verb.

Then give the user a short table of each row, verb and the values written. Include:

- Any form you left blank, and why.
- A reminder that the cards change only after the next `./sync push`.
