<script lang="ts">
  import { page } from '$app/state';

  let notFound = $derived(page.status === 404);
</script>

<svelte:head>
  <title>{notFound ? 'Page not found' : 'Something went wrong'} · Tennis Connect</title>
</svelte:head>

<section class="page narrow stack">
  <header class="page-head">
    <div>
      <p class="kicker">Error {page.status}</p>
      <h1>{notFound ? 'Page not found' : 'Something went wrong'}</h1>
    </div>
  </header>
  <div class="sheet-card stack">
    <p class="mb-0">
      {#if notFound}
        That page has moved, closed, or never existed. Your card and matches are still safe.
      {:else}
        We hit an unexpected problem. Try the page again, or head back to somewhere familiar.
      {/if}
    </p>
    <div class="d-flex flex-wrap gap-2">
      <a class="btn btn-primary" href="/">Back to Tennis Connect</a>
      <a class="btn btn-outline-secondary" href="/dashboard">Go to dashboard</a>
    </div>
  </div>
</section>
