[![Contributors][contributors-shield]][contributors-url]
[![Issues][issues-shield]][issues-url]
[![MIT License][license-shield]][license-url]
[![LinkedIn][linkedin-shield]][linkedin-url]

<br />
<div align="center">

<h3 align="center">Vault City</h3>

  <p align="center">
    Walk through your Obsidian vault as a 3D city
    <br />
    <a href="https://github.com/yuriykis/vault-city/issues">Report Bug</a>
  </p>
</div>


## Important Note
This project is in early development. Things will change, and some may break.

## About The Project
Vault City reads the notes in an Obsidian vault and builds a city out of them, which you walk through in first person. Every note is a building: the more notes link to it, the taller it stands. Notes that share a topic form a district, and each note's category gives its building a color.

The game only reads the vault. It never writes, moves or deletes anything in it.

## Features
* Districts: notes with the same `topic` share a square block. A note named like the topic stands in its center as the hub. Notes without a topic make up the suburbs.
* Heights: a building grows with the number of `[[wikilinks]]` pointing at its note, on a log scale, so one very popular note doesn't dwarf the city.
* Colors: each `category` gets its own color from a color-blind-safe palette.
* Labels: look at a building to see its note's name, and press Space to read the start of the note.
* Collisions: buildings are solid, so you walk the streets and alleys.
* Fog: distant buildings fade into the sky.
* Polish and Cyrillic note names are drawn with the system Arial font on macOS.

## How Your Vault Is Read
Only Markdown files directly inside the vault's `Notes/` folder become buildings. Two optional frontmatter properties shape the city:

```yaml
---
topic: "[[Go]]"        # the district; plain text or a quoted link
category: Concept      # the building's color
---
```

## Prerequisites
* Go 1.26 or newer
* A C compiler for [raylib-go](https://github.com/gen2brain/raylib-go) (on macOS, the Xcode Command Line Tools)
* [Mage](https://magefile.org), optional, for the `mage` commands below

## Getting Started

1. Clone the repo
   ```sh
   git clone https://github.com/yuriykis/vault-city.git
   cd vault-city
   ```
2. Point the game at your vault
   ```sh
   cp .env.example .env
   ```
   Then set `VAULT_PATH` in `.env` to your Obsidian vault.
3. Run it
   ```sh
   mage run
   ```
   Or without Mage, passing the vault directly:
   ```sh
   go run . -vault /path/to/your/vault
   ```
4. Build a binary into `bin/vault-city`
   ```sh
   mage build
   ```

## Controls
| Key | Action |
| --- | --- |
| W A S D | Walk |
| Mouse | Look around |
| Space | Switch the label between the note's name and its text |
| 1 / 2 / 3 / 4 | Free, first-person, third-person or orbital camera |
| P | Switch between perspective and orthographic view |
| Esc | Quit |

## License
Distributed under the MIT License. See `LICENSE` for more information.


<!-- MARKDOWN LINKS & IMAGES -->
<!-- https://www.markdownguide.org/basic-syntax/#reference-style-links -->
[contributors-shield]: https://img.shields.io/github/contributors/yuriykis/vault-city.svg?style=for-the-badge
[contributors-url]: https://github.com/yuriykis/vault-city/graphs/contributors
[issues-shield]: https://img.shields.io/github/issues/yuriykis/vault-city.svg?style=for-the-badge
[issues-url]: https://github.com/yuriykis/vault-city/issues
[license-shield]: https://img.shields.io/github/license/yuriykis/vault-city.svg?style=for-the-badge
[license-url]: https://github.com/yuriykis/vault-city/blob/main/LICENSE
[linkedin-shield]: https://img.shields.io/badge/-LinkedIn-black.svg?style=for-the-badge&logo=linkedin&colorB=555
[linkedin-url]: https://linkedin.com/in/yuriy-kis
