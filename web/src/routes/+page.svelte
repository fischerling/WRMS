<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import addSVG from '../svg/add.svg';
	import playSVG from '../svg/play.svg';
	import pauseSVG from '../svg/pause.svg';
	import type { WsMessage, Song, SearchResponse } from '../lib/api';
	import API from '../lib/api';
	API.baseUrl = '/';

	let status: 'paused' | 'playing' = 'paused';
	let currentSong: Song | null = null;
	let playlist: Array<Song> = [];
	let votes: Map<string, 'up' | 'down'> = new Map();

	let search = '';
	let searchResults: Array<Song> = [];

	let socket: WebSocket;
	onMount(function () {
		socket = new WebSocket('ws://localhost:8080/ws');
		// Expose websocket for debugging
		// ts-ignore
		window.WS = socket;
		console.info('Attempting Connection...');

		socket.onopen = () => {
			console.info('Successfully Connected');
		};

		socket.onclose = (event) => {
			console.info('Socket Closed Connection: ', event);
		};

		socket.onerror = (error) => {
			console.error('Socket Error: ', error);
		};

		socket.onmessage = ({ data }) => {
			const { cmd, songs }: WsMessage = JSON.parse(data);
			console.debug('Ws Message', cmd, songs);
			switch (cmd) {
				case 'add':
					playlist = [...playlist, ...songs];
					break;
				case 'update':
					for (const song of songs) {
						playlist[
							playlist.findIndex(({ uri }) => {
								return uri === song.uri;
							})
						] = song;
					}
					break;
				case 'pause':
					status = 'paused';
					break;
				case 'play':
					status = 'playing';
					currentSong = songs[0];
					playlist = playlist.filter(({ uri }) => uri !== currentSong?.uri);
					break;
				case 'upvoted':
					for (const { uri } of songs) {
						votes.set(uri, 'up');
					}
					votes = votes;
					break;
				case 'downvoted':
					for (const { uri } of songs) {
						votes.set(uri, 'down');
					}
					votes = votes;
					break;
			}
		};
	});
	onDestroy(function () {
		socket?.close();
	});

	$: playlist = playlist.sort(function (a, b) {
		return b.weight - a.weight;
	});

	async function submitSearch() {
		const response = await API.get('search', { pattern: search });
		const { songs }: SearchResponse = await response.json();
		searchResults = songs;
	}

	function vote(direction: 'up' | 'down', uri: string) {
		return async function () {
			let action;
			if (votes.get(uri) === direction) {
				votes.delete(uri);
				action = 'unvote';
			} else {
				votes.set(uri, direction);
				action = direction;
			}
			const response = await API.get(action, { song: uri });
			console.debug(response);
		};
	}
</script>

<div class="content">
	<form on:submit|preventDefault={submitSearch}>
		<h1>Add your song</h1>
		<label><input bind:value={search} id="search-bar" placeholder="Title/Artist/Album/..." /></label
		>
		<button formaction="submit">Search</button>
	</form>
	<div class="list">
		{#each searchResults as { title, artist, source, uri, weight } (uri)}
			<div class="flex-row search-results">
				<button
					class="icon-button"
					on:click={() =>
						API.post('add', { title, artist, source, uri, weight }).then(console.debug)}
				>
					<img src={addSVG} alt="Add" />
				</button>
				{artist + ' - ' + title}
				<small>{source}</small>
			</div>
		{/each}
	</div>
	<h1>Playlist</h1>
	<div class="list">
		{#each playlist as { title, artist, source, uri, weight } (uri)}
			<div>
				<button class="vote" on:click={vote('up', uri)}>Up</button>
				<button class="vote" on:click={vote('down', uri)}>Down</button>
				{`${weight} ${artist} - ${title}`}
				<small>{`(${source})`}</small>
			</div>
		{/each}
	</div>
</div>

<div class="player flex-row">
	<button class="icon-button" on:click={() => API.get('playpause').then(console.debug)}>
		{#if status === 'playing'}
			<img src={pauseSVG} alt="Pause" />
		{:else}
			<img src={playSVG} alt="Play" />
		{/if}
	</button>
	<p>
		{#if currentSong !== null}
			{currentSong.title} <br /> <small>{currentSong.artist}</small>
		{:else}
			No songs schedulded :( <br /> <a href="#search-bar">Try adding some.</a>
		{/if}
	</p>
</div>

<style>
	.content {
		width: 100vw;
		height: calc(100vh - 5em);
	}

	@media (max-width: 50em) {
		.content {
			overflow-y: scroll;
			display: flex;
			flex-direction: column;
				align-items: center;
		}
	}
	@media not (max-width: 50em) {
		.content {
			display: grid;
			grid-template-columns: 1fr 1fr;
			grid-template-rows: min-content 1fr;
		}
		.content > * {
			grid-row: 1;
		}
		.content > form {
			grid-column: 1;
		}
		.content > h1 {
			grid-column: 2;
		}

		.content > .list {
			grid-row: 2;
		}
		.content > div.list:first-child {
			grid-column: 1;
		}
		.content > div.list:last-child {
			grid-column: 2;
		}

		.list {
			overflow-y: scroll;
		}
	}

	.player {
		position: fixed;
		bottom: 0;
		width: 100%;
		height: 5em;
		border-top: 1px solid white;
	}

	.search-results > button {
		--icon-size: 2em;
	}
	.search-results > small {
		color: var(--secondary);
		margin-left: 0.5em;
	}
</style>
