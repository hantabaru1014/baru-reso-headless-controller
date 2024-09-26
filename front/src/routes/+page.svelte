<script lang="ts">
	import { createPromiseClient } from '@connectrpc/connect';
	import { createConnectTransport } from '@connectrpc/connect-web';
	import { UserService } from '$lib/pbgen/hdlctrl/v1/user_connect';
	import { ControllerService } from '$lib/pbgen/hdlctrl/v1/controller_connect';
	import type { HeadlessHost } from '$lib/pbgen/hdlctrl/v1/controller_pb';

	const transport = createConnectTransport({
		baseUrl: '/'
	});
	const userServiceClient = createPromiseClient(UserService, transport);
	const ctrlServiceClient = createPromiseClient(ControllerService, transport);

	let apiKey = '';
	let hosts: HeadlessHost[] = [];

	const fetchToken = async () => {
		const res = await userServiceClient.getTokenByAPIKey({
			apiKey
		});

		return res.token;
	};

	const fetchHosts = async () => {
		const token = await fetchToken();
		const res = await ctrlServiceClient.listHeadlessHost(
			{},
			{
				headers: [['authorization', `Bearer ${token}`]]
			}
		);

		hosts = res.hosts;
	};
</script>

<h1>Welcome to baru-reso-headless</h1>

<span>Enter your API key:</span>
<input bind:value={apiKey} />

<button on:click={fetchHosts}>Fetch hosts</button>

<h2>Hosts</h2>

<ul>
	{#each hosts as host}
		<li>{host.id} : {host.name}</li>
	{/each}
</ul>

{#each new Array(20).map((_, i) => i) as i}
	<div class="mt-5">
		クソ長コンテンツ {i}
	</div>
{/each}
