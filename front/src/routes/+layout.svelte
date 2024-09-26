<script lang="ts">
	import { Styles } from '@sveltestrap/sveltestrap';
	import { Navbar, NavbarBrand } from '@sveltestrap/sveltestrap';
	import SideBar from './SideBar.svelte';
</script>

<Styles theme={'light'} />

<div class="root-container">
	<Navbar color="light">
		<NavbarBrand href="/">BRHDL</NavbarBrand>
	</Navbar>
	<div class="content-container">
		<div class="sidebar bg-body-secondary">
			<SideBar />
		</div>
		<main class="main-content contaner-xl px-md-4 py-4">
			<slot />
		</main>
	</div>
</div>

<style>
	.root-container {
		display: flex;
		flex-direction: column;
		flex-wrap: wrap;
		width: 100vw;
		height: 100vh;
	}

	.content-container {
		flex-basis: 0;
		height: 0;
		flex-grow: 1;
		overflow: auto;
		display: inline-flex;
		flex-direction: row;
	}

	.sidebar {
		width: 10rem;
	}

	.main-content {
		flex-basis: 0;
		width: 0;
		flex-grow: 1;
		overflow: auto;
		display: block;
	}
</style>
