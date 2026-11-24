<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON, relativeDate, type Match } from '#lib/api';

  let matches: Match[] = [];
  let loading = true;
  let error = '';

  onMount(async () => {
    try {
      matches = (await apiJSON<{ matches: Match[] }>('/api/v1/matches')).matches;
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'Matches are unavailable.';
    } finally {
      loading = false;
    }
  });
</script>

<svelte:head>
  <title>Matches · Tennis Connect</title>
</svelte:head>

<section class="page stack">
  <div>
    <p class="score"><span class="tennis-ball me-2" aria-hidden="true"></span>Mutual</p>
    <h1>Matches</h1>
  </div>

  {#if loading}
    <p role="status">Loading matches…</p>
  {:else if error}
    <p class="alert-box" role="alert">{error}</p>
  {:else if !matches.length}
    <div class="surface stack">
      <h2>No matches yet</h2>
      <p class="muted">A match appears only when you and another player both choose Like.</p>
      <a class="btn btn-primary align-self-start" href="/discover">Go to Discover</a>
    </div>
  {:else}
    <ul class="match-list">
      {#each matches as match (match.match_id)}
        <li class="card p-2">
          <a href={`/matches/${match.match_id}`} class="match-link">
            {#if match.photo_urls.length}
              <img class="photo thumb" src={match.photo_urls[0]} alt="" />
            {:else}
              <div class="photo thumb placeholder" aria-hidden="true"></div>
            {/if}
            <span class="match-copy">
              <strong>{match.display_name}</strong>
              <span class="muted">Matched {relativeDate(match.matched_at)}</span>
              <span class="chip" class:optic={match.has_instagram}><i class="bi bi-instagram me-1" aria-hidden="true"></i>{match.has_instagram ? 'Instagram available' : 'No Instagram'}</span>
            </span>
          </a>
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  .match-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.75rem;
  }

  .match-link {
    display: flex;
    align-items: center;
    gap: 1rem;
    color: var(--ink);
    text-decoration: none;
  }

  .thumb {
    width: 5rem;
    flex: 0 0 5rem;
  }

  .placeholder {
    background: #e5eae7;
  }

  .match-copy {
    display: grid;
    gap: 0.25rem;
  }
</style>
