---
name: update-examples-notes
description: Review and improve the Examples and Notes columns (E and F of the `words` tab) for a row or range of rows in the Greek vocabulary Google Sheet that feeds this Anki deck. Use this whenever the user wants example sentences or notes checked, fixed, improved, added or updated for specific sheet rows, such as "update examples for rows 120-140" or "fix the notes on row 88", or asks whether a card's example sentence is right, even if they don't name the columns.
---

# Update examples and notes

Columns E and F are shown on Anki cards and are studied as correct Greek, so a mistake here gets memorized. Almost every row already has text the user may have learned, so this is a review, not a rewrite: change only what makes the card better, and show every change before writing it.

## 1. Find the sheet

Read `google_sheet_id` and `sheet_name` from `~/.config/sync/config.yaml`. This repo is public, so the ID stays in that file rather than in this skill.

Load the Google Sheets `get_values` and `update_values` tools. If they're missing, ask the user to enable the Google Sheets connector and stop.

Read `words!C1:H1`. Expect `english`, `greek`, `examples`, `notes`, `part of speech`, `attributes`. If the columns have moved, stop and tell the user: everything below assumes these positions.

## 2. Choose the rows

The user gives a row (`120`), a range (`120-140`), or a mix (`88, 120-140`). If they give none, ask which rows they mean. Don't scan the whole sheet: nothing marks a row as needing work, so a sweep would just produce a huge review.

Read `words!C<first>:P<last>` for each span. Skip row 1 and any row with no Greek in D, and mention any you skipped. Columns C, D, G and H say what the word means and what it is. M–P hold verb forms the card already shows.

If you'll need to write a new sentence, also read `words!D:D` so you can prefer words already in the deck.

## 3. Review each row

Leave a row alone if it's fine, even if you'd have worded it differently. A rewrite for taste costs the user a sentence they may already know. Many rows will come back unchanged, and that's a good result.

If you're not sure something is wrong, don't change it. Raise it in the preview instead.

### E: examples

Change an example only if:

- The Greek is wrong: spelling, accent, agreement, case, article, or verb form.
- The translation doesn't match the Greek.
- It doesn't use the word in the meaning given in column C.
- It's unnatural, such as an English phrasing carried over word for word.

An odd sentence is not a defect. Some come from the drills in the `sentences` tab, such as an owl listening to the radio, and they're meant to be memorable.

Follow this format, because the sync code depends on it:

- One example per line, written as `Greek sentence — English translation`.
- The separator is exactly ` — `: an em dash with a space on each side. The card splits each line on it, and a line without it loses its translation.
- Greek sentences are short and everyday, usually about 5 words and at most 12, with the capital and final punctuation of a full sentence.
- The headword appears in the sentence, inflected as needed.
- For Greek questions, use the Greek question mark `;`.
- Use one line. Add a second only for a separate meaning or a fixed phrase worth knowing, such as χαρά's `Με χαρά! — With pleasure!`. Only 15 of 942 rows have two.
- Prefer words already in the deck, because a sentence full of new words is harder to study.

Explanations belong in Notes, never in Examples.

### F: notes

Blank is fine: about 4 in 10 rows have no note. Add one only if it tells the learner something the card doesn't, and change an existing note only if it's wrong or misleading. The card already shows the noun's article and gender, and a verb's future and aorist, so don't add notes that repeat those.

Keep to one short line, usually about 50 characters and at most about 140. Existing notes state the fact directly, end with a period, and gloss Greek in parentheses. These are the kinds of note that earn their place:

| Kind | Example |
|---|---|
| Irregular form | `Irregular plural: οι πόλεις.` |
| Variant spelling | `Also spelled χτες.` |
| Another meaning | `Also a computer mouse.` |
| Paired word | `Father-in-law is ο πεθερός.` |
| Easily confused word | `Adjective. The adverb "near" is κοντά.` |
| Grammar it requires | `Takes the genitive.` |
| Everyday usage | `Everyday speech often uses Νοέμβρης.` |
| Phrase | `Also "tight" for clothes; στενός φίλος (close friend).` |

## 4. Preview and wait

Don't write anything yet. Show each row that would change, with its before text, after text and a one-line reason:

```
Row 100: πόλη (city)
F before: (blank)
F after:  Irregular plural: οι πόλεις.
Why: the plural isn't predictable from the singular.
```

Then give:

- How many rows are unchanged.
- Anything you were unsure about and left as it is.
- Any rows you skipped.

Ask the user to approve. They may approve everything, drop some rows, or reword some. Write only what they approved. If nothing needs changing, say so and stop.

For a long range, keep the preview to the rows that change so it stays easy to read.

## 5. Write

Sheets may have been re-sorted during the review, so first re-read D for the whole span and confirm every row still holds the same word. If any differ, stop and tell the user.

Write each span in one `update_values` call to `words!E<first>:F<last>`, with one `[E, F]` pair per row. Send `null` for every cell that isn't changing, including all of a row with no change: `null` skips a cell, while `""` clears it. Use a real newline between two example lines.

```json
{"range": "words!E100:F102", "values": [[null, "Irregular plural: οι πόλεις."], [null, null], ["Πού είναι η τράπεζα; — Where is the bank?", null]]}
```

Don't run `./sync push`. The user decides when to push to Anki.

## 6. Verify and report

Read `words!D<first>:F<last>` back and check that the approved text landed on the right rows and nothing else moved.

Then tell the user how many rows changed, and remind them the cards change only after the next `./sync push`.
