export type Song = {
	title: string;
	artist: string;
	source: string;
	uri: string;
	weight: number;
};

export type WsMessage = {
	cmd: 'add' | 'update' | 'pause' | 'play' | 'upvoted' | 'downvoted';
	songs: Array<Song>;
};
export type SearchResponse = {
	cmd: 'search';
	songs: Array<Song>;
};

const HEADERS = new Headers([
	['Accept', 'application/json'],
	['Content-Type', 'application/json']
]);
const API = {
	baseUrl: '',
	get(url: string, body?: Record<string, string>) {
		if (body !== undefined) {
			url += '?' + new URLSearchParams(body);
		}
		return fetch(this.baseUrl + url, {
			method: 'GET'
		});
	},
	post(url: string, body?: object) {
		return fetch(this.baseUrl + url, {
			method: 'POST',
			body: body && JSON.stringify(body),
			headers: HEADERS
		});
	}
};
export default API;
