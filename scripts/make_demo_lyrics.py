"""Write demo .lrc files next to every demo track (idempotent).

Timings are spread across the real track duration read from the SQLite library,
so the lyric highlighting lines up with playback.
"""

import os
import random
import sqlite3

ROOT = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "data")

POOL = [
    "Streetlights bleeding through the blinds",
    "I keep the radio on low",
    "Every window holds a stranger's night",
    "We were never good at saying slow",
    "Somewhere a train is leaving town",
    "You said the quiet was a kind of sound",
    "Hold the door, the morning's coming through",
    "Nothing here is lost, it's only moved",
    "Paper cups and borrowed time",
    "The tape runs out before the line",
    "I traced the map across your hand",
    "Static blooming in the dark",
    "We drove until the road gave out",
    "And called the empty streets our home",
    "There's a colour in the rain",
    "That I only learned to name",
    "Falling slow like afternoon",
    "Everything arrives too soon",
    "Turn the volume down a little more",
    "I can hear the floorboards breathe",
    "Somewhere past the harbour lights",
    "The water keeps what it receives",
    "Let the record skip a beat",
    "It sounds better incomplete",
    "All the words we left unsaid",
    "Are sleeping underneath the bed",
    "Blue hour, blue hour, don't go",
    "Stay until the shadows grow",
    "We are only passing through",
    "Made of light and residue",
    "Count the cars along the bridge",
    "Each one carrying a wish",
    "If you're listening tonight",
    "Leave a light on, leave it bright",
    "I don't need to know the end",
    "I just want to start again",
    "Slow down, slow down, we're almost there",
    "Nothing waiting, nothing to repair",
    "The city folds into the sea",
    "And leaves a little room for me",
]

CHORUS = [
    "And it goes on, and it goes on",
    "Holding on to what is gone",
    "Every echo finds a wall",
    "And the quiet takes it all",
]


def fmt(sec: float) -> str:
    m = int(sec // 60)
    s = sec - m * 60
    return f"[{m:02d}:{s:05.2f}]"


def main() -> None:
    db = sqlite3.connect(os.path.join(ROOT, "mrome.db"))
    rows = db.execute("SELECT path, title, duration FROM songs").fetchall()
    made = 0
    for path, title, duration in rows:
        if not path or not os.path.exists(path):
            continue
        lrc_path = os.path.splitext(path)[0] + ".lrc"
        if os.path.exists(lrc_path):
            continue
        rnd = random.Random(title)
        dur = max(30, float(duration or 45))
        lines = rnd.sample(POOL, 6)
        verse = lines[:3]
        chorus = rnd.sample(CHORUS, 2)
        verse2 = lines[3:6]
        body = verse + chorus + verse2 + [chorus[0]]
        head = dur * 0.06
        step = (dur * 0.86) / len(body)
        out = [fmt(head + i * step) + line for i, line in enumerate(body)]
        with open(lrc_path, "w", encoding="utf-8") as fh:
            fh.write("\n".join(out) + "\n")
        made += 1
    print(f"wrote {made} lyric files")


if __name__ == "__main__":
    main()
