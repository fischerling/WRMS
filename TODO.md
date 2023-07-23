## General

- [X] add admin
  - [X] allow admin to delete songs
  - [X] allow admin to skip songs
- [X] continue playback when song is added to empty playlist
- [ ] support advanced search
  - [X] client
  - [ ] spotify
  - [ ] local
- [X] Show more Song information
  - [ ] Fetch and show cover art
  - [X] Show a song's Album

## Fronted

- [X] escape HTML 
- [X] support incremental search results
  - [X] distinguish different search queries
- [X] do not apply obsolete events
- [X] support timeBonus

## Web Backend

- [X] Implement an event endpoint

## Backends

- [ ] cancel old searches
- [ ] make wrms thread safe
  - [ ] synchronize Player.runMpv with the rest of Wrms
  - [X] tag events with ids
- [X] paralyze search
  - [X] report results incrementally
  - [X] distinguish different search queries
- [X] Implement local storage backend
  - [X] Implement upload
    - [X] Remove uploaded songs after they were played
- [X] Fix youtube search
- [ ] Support loading playlists
  - [ ] m3u
  - [ ] spotify playlist

## Configuration

- [X] Implement command line flags
- [X] Implement config file support
