"""
Generate a demo music library: tagged MP3s with procedurally painted cover art.
Usage: python scripts/make_demo_library.py [target_dir]
"""
import io
import math
import os
import random
import sys

import lameenc
import numpy as np
from PIL import Image, ImageDraw, ImageFilter, ImageFont
from mutagen.id3 import APIC, ID3, TALB, TCON, TDRC, TIT2, TPE1, TPE2, TRCK, TYER

SR = 44100
random.seed(7)
np.random.seed(7)

ALBUMS = [
    {
        "artist": "Aria Vale", "album": "Neon Meridian", "year": 2024, "genre": "Electronic",
        "colors": ("#6D5EF8", "#B06AB3", "#0B0A1F"),
        "tracks": ["Midnight Transit", "Neon Meridian", "Glass Elevator", "Afterglow Protocol"],
        "bpm": 96, "root": 55.0,
    },
    {
        "artist": "Kaito Mori", "album": "Tidal Memory", "year": 2023, "genre": "Ambient",
        "colors": ("#0FB9A6", "#1B4D3E", "#04120F"),
        "tracks": ["Saltwater Drift", "Low Tide Letters", "Harbour Lights"],
        "bpm": 72, "root": 49.0,
    },
    {
        "artist": "Lena Ost", "album": "Amber Rooms", "year": 2022, "genre": "Indie Pop",
        "colors": ("#F2A65A", "#C4553B", "#20100A"),
        "tracks": ["Amber Rooms", "Paper Lanterns", "Sunday Static"],
        "bpm": 104, "root": 65.4,
    },
    {
        "artist": "Northbound", "album": "Blue Hour Signals", "year": 2025, "genre": "Post Rock",
        "colors": ("#2E6BFF", "#62D0FF", "#050B1E"),
        "tracks": ["Signal Fires", "Blue Hour", "Twelve Kilometres"],
        "bpm": 88, "root": 58.3,
    },
    {
        "artist": "Mira Sol", "album": "Velvet Static", "year": 2021, "genre": "Dream Pop",
        "colors": ("#E0559B", "#7A2E8E", "#170616"),
        "tracks": ["Velvet Static", "Slow Bloom", "Cassette Sun"],
        "bpm": 84, "root": 61.7,
    },
    {
        "artist": "The Quiet Field", "album": "Dust & Olive", "year": 2020, "genre": "Folk",
        "colors": ("#C9B458", "#6B6B3A", "#141308"),
        "tracks": ["Dust & Olive", "Woodsmoke", "The Long Way Home"],
        "bpm": 76, "root": 52.0,
    },
    {
        "artist": "Ren Ito", "album": "Concrete Sky", "year": 2024, "genre": "Lo-Fi",
        "colors": ("#6E8CA0", "#2C3E50", "#080B0F"),
        "tracks": ["Concrete Sky", "Station 4AM", "Rain on Steel"],
        "bpm": 80, "root": 46.2,
    },
]


def hex_rgb(h):
    h = h.lstrip("#")
    return tuple(int(h[i:i + 2], 16) for i in (0, 2, 4))


def make_cover(album, size=900):
    c1, c2, c3 = (hex_rgb(x) for x in album["colors"])
    # base radial wash
    y, x = np.mgrid[0:size, 0:size].astype(np.float32)
    cx, cy = size * random.uniform(0.3, 0.7), size * random.uniform(0.25, 0.6)
    d = np.sqrt((x - cx) ** 2 + (y - cy) ** 2) / (size * 0.9)
    d = np.clip(d, 0, 1)[..., None]
    img = np.array(c1, dtype=np.float32) * (1 - d) + np.array(c3, dtype=np.float32) * d
    # second light source
    cx2, cy2 = size * random.uniform(0.2, 0.9), size * random.uniform(0.5, 1.0)
    d2 = np.sqrt((x - cx2) ** 2 + (y - cy2) ** 2) / (size * 1.1)
    d2 = np.clip(1 - d2, 0, 1)[..., None] ** 2.2
    img = img * (1 - 0.75 * d2) + np.array(c2, dtype=np.float32) * 0.75 * d2

    # grain
    grain = np.random.normal(0, 7, (size, size, 1))
    img = np.clip(img + grain, 0, 255).astype(np.uint8)
    base = Image.fromarray(img).convert("RGBA")

    # concentric rings
    overlay = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    dr = ImageDraw.Draw(overlay)
    for i in range(18):
        r = size * (0.16 + i * 0.031)
        alpha = int(26 * math.exp(-i / 7))
        dr.ellipse([cx - r, cy - r, cx + r, cy + r], outline=(255, 255, 255, alpha), width=max(1, int(size * 0.0022)))
    overlay = overlay.filter(ImageFilter.GaussianBlur(size * 0.0016))
    base = Image.alpha_composite(base, overlay)

    # horizon band
    band = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    bd = ImageDraw.Draw(band)
    hy = int(size * random.uniform(0.58, 0.7))
    bd.rectangle([0, hy, size, hy + int(size * 0.012)], fill=(255, 255, 255, 30))
    band = band.filter(ImageFilter.GaussianBlur(size * 0.006))
    base = Image.alpha_composite(base, band)

    # soft vignette
    vig = Image.new("L", (size, size), 0)
    vd = ImageDraw.Draw(vig)
    vd.ellipse([-size * 0.25, -size * 0.3, size * 1.25, size * 1.2], fill=255)
    vig = vig.filter(ImageFilter.GaussianBlur(size * 0.08))
    dark = Image.new("RGBA", (size, size), (0, 0, 0, 110))
    base = Image.composite(base, Image.alpha_composite(base, dark), vig)

    # typography
    draw = ImageDraw.Draw(base)
    font_dir = "C:/Windows/Fonts"
    try:
        title_font = ImageFont.truetype(os.path.join(font_dir, "georgia.ttf"), int(size * 0.075))
        artist_font = ImageFont.truetype(os.path.join(font_dir, "arial.ttf"), int(size * 0.026))
    except Exception:
        title_font = ImageFont.load_default()
        artist_font = ImageFont.load_default()
    pad = int(size * 0.075)
    draw.text((pad, size - pad - int(size * 0.11)), album["album"], font=title_font,
              fill=(255, 255, 255, 240))
    draw.text((pad + 2, size - pad - int(size * 0.035)), album["artist"].upper(), font=artist_font,
              fill=(255, 255, 255, 150))
    return base.convert("RGB")


def synth_track(duration, root, bpm, seed):
    rng = np.random.default_rng(seed)
    n = int(SR * duration)
    t = np.arange(n) / SR
    out = np.zeros((n, 2), dtype=np.float32)

    # chord: root, minor 7th, 9th, 11th
    ratios = [1.0, 1.1892, 1.4983, 2.0, 2.3784]
    for k, ratio in enumerate(ratios):
        f = root * ratio * (2 if k == 4 else 1)
        detune = 0.0015 * rng.uniform(-1, 1)
        lfo = 1 + 0.0025 * np.sin(2 * np.pi * (0.07 + 0.02 * k) * t + k)
        amp = 0.16 / (1 + 0.35 * k)
        for ch, phase in enumerate([0.0, 0.5]):
            wave = np.sin(2 * np.pi * f * (1 + detune) * lfo * t + phase)
            wave += 0.22 * np.sin(2 * np.pi * f * 2 * lfo * t + phase * 1.7)
            out[:, ch] += amp * wave.astype(np.float32)

    # slow amplitude breathing
    breath = 0.72 + 0.28 * np.sin(2 * np.pi * 0.055 * t + 1.2)
    out *= breath[:, None]

    # bell arpeggio
    step = 60.0 / bpm
    notes = [1.0, 1.4983, 1.1892, 2.0]
    idx = 0
    tt = 0.0
    while tt < duration - 2:
        f = root * 4 * notes[idx % len(notes)]
        start = int(tt * SR)
        length = min(int(1.8 * SR), n - start)
        if length > 0:
            local = np.arange(length) / SR
            env = np.exp(-local * 3.1) * (1 - np.exp(-local * 90))
            tone = np.sin(2 * np.pi * f * local) + 0.3 * np.sin(2 * np.pi * f * 2.01 * local)
            pan = 0.5 + 0.32 * math.sin(idx * 1.7)
            out[start:start + length, 0] += (0.05 * env * tone * (1 - pan)).astype(np.float32)
            out[start:start + length, 1] += (0.05 * env * tone * pan).astype(np.float32)
        idx += 1
        tt += step * (1.5 if idx % 3 == 0 else 1.0)

    # airy noise
    noise = rng.normal(0, 1, n).astype(np.float32)
    kernel = np.ones(220, dtype=np.float32) / 220
    noise = np.convolve(noise, kernel, mode="same")
    out[:, 0] += noise * 0.018
    out[:, 1] += np.roll(noise, 900) * 0.018

    # fade in/out
    fade = int(2.4 * SR)
    env = np.ones(n, dtype=np.float32)
    env[:fade] = np.linspace(0, 1, fade) ** 1.6
    env[-fade:] = np.linspace(1, 0, fade) ** 1.4
    out *= env[:, None]

    peak = np.max(np.abs(out)) or 1.0
    out = np.tanh(out / peak * 1.4) * 0.72
    return (out * 32767).astype(np.int16)


def encode_mp3(pcm, path):
    enc = lameenc.Encoder()
    enc.set_bit_rate(192)
    enc.set_in_sample_rate(SR)
    enc.set_channels(2)
    enc.set_quality(2)
    data = enc.encode(pcm.tobytes()) + enc.flush()
    with open(path, "wb") as f:
        f.write(data)


def tag_mp3(path, album, title, track, cover_bytes):
    tags = ID3()
    tags.add(TIT2(encoding=3, text=title))
    tags.add(TPE1(encoding=3, text=album["artist"]))
    tags.add(TPE2(encoding=3, text=album["artist"]))
    tags.add(TALB(encoding=3, text=album["album"]))
    tags.add(TCON(encoding=3, text=album["genre"]))
    tags.add(TYER(encoding=3, text=str(album["year"])))
    tags.add(TDRC(encoding=3, text=str(album["year"])))
    tags.add(TRCK(encoding=3, text=f"{track}/{len(album['tracks'])}"))
    tags.add(APIC(encoding=3, mime="image/jpeg", type=3, desc="Cover", data=cover_bytes))
    tags.save(path, v2_version=3)


def main():
    target = sys.argv[1] if len(sys.argv) > 1 else "data/music"
    os.makedirs(target, exist_ok=True)
    lrc = """[00:00.00]Midnight transit, windows down
[00:08.50]The city hums in violet sound
[00:17.00]Every light a tiny sun
[00:25.50]We are the afterglow, we are the run
[00:34.00]Neon meridian, carry me home
[00:42.50]Past the stations, past the stone
[00:51.00]If the morning takes it slow
[00:59.50]Leave a little light to glow
"""
    for album in ALBUMS:
        folder = os.path.join(target, album["artist"], album["album"])
        os.makedirs(folder, exist_ok=True)
        cover = make_cover(album)
        buf = io.BytesIO()
        cover.save(buf, format="JPEG", quality=92)
        cover_bytes = buf.getvalue()
        cover.save(os.path.join(folder, "cover.jpg"), quality=92)
        for i, title in enumerate(album["tracks"], start=1):
            duration = 38 + (i * 7) % 22
            pcm = synth_track(duration, album["root"] * (1 + 0.06 * (i - 1)), album["bpm"], seed=hash(title) % 9999)
            path = os.path.join(folder, f"{i:02d} - {title}.mp3")
            encode_mp3(pcm, path)
            tag_mp3(path, album, title, i, cover_bytes)
            if album["album"] == "Neon Meridian" and i == 1:
                with open(os.path.join(folder, "01 - Midnight Transit.lrc"), "w", encoding="utf-8") as f:
                    f.write(lrc)
            print("wrote", path)

    # An untagged file to exercise the unknown-artist fallback.
    pcm = synth_track(30, 43.65, 70, seed=42)
    path = os.path.join(target, "Untagged Sketch.mp3")
    encode_mp3(pcm, path)
    print("wrote", path)


if __name__ == "__main__":
    main()
