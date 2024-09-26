<script lang="ts">
	import {
		Nav,
		NavItem,
		NavLink,
	} from '@sveltestrap/sveltestrap';
</script>

<nav class="px-2">
	<Nav navbar>
		<NavItem>
			<NavLink href="/">Home</NavLink>
		</NavItem>
		<NavItem>
			<NavLink href="/hosts">Hosts</NavLink>
		</NavItem>
		<NavItem>
			<NavLink href="/sessions">Sessions</NavLink>
		</NavItem>
	</Nav>
</nav>

<style>
	:global(.sidebar .nav-link) {
		color: #333;
	}

	:global(.sidebar .nav-link.active) {
		color: #0d6efd;
	}
</style>
