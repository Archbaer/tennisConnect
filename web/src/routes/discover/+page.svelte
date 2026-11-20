<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { ApiError, apiJSON, distanceLabel, type DiscoveryCard } from '#lib/api';

  let cards: DiscoveryCard[] = [];
  let cursor: string | null = null;
  let radius = 10;
  let loading = true;
  let error = '';
  let blockedReason = '';
  let matchID: string | null = null;
  let confirmPass: string | null = null;
  let acting: string | null = null;

  onMount(load);

  async function load(reset = true) {
    loading = true;
    error = '';
    blockedReason = '';
    try {
      const query = new URLSearchParams({ radius: String(radius), limit: '20' });
      if (!reset && cursor) query.set('cursor', cursor);
      const page = await apiJSON<{ cards: DiscoveryCard[]; next_cursor: string | null }>(
        `/api/v1/discovery?${query}`
      );
      cards = reset ? page.cards : [...cards, ...page.cards];
      cursor = page.next_cursor;
    } catch (problem) {
      if (problem instanceof ApiError && problem.status === 409) {
        blockedReason = problem.message;
      } else {
        error = problem instanceof Error ? problem.message : 'Discover is unavailable.';
      }
    } finally {
      loading = false;
    }
  }

  async function react(targetID: string, action: 'like' | 'pass') {
    acting = targetID;
    error = '';
    try {
      const result = await apiJSON<{ matched: boolean; match_id?: string }>('/api/v1/reactions', {
        method: 'POST',
        body: JSON.stringify({ target_id: targetID, action })
      });
      cards = cards.filter((card) => card.player.player_id !== targetID);
      confirmPass = null;
      if (result.matched && result.match_id) {
        matchID = result.match_id;
      }
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'That reaction did not save.';
    } finally {
      acting = null;
    }
  }
</script>

<svelte:head>
  <title>Discover · Tennis Connect</title>
</svelte:head>

<section class="page stack">
  <div class="row spread">
    <div>
      <p class="score">Discover</p>
      <h1>Players near you</h1>
    </div>
    <label class="radius">
      Radius
      <select bind:value={radius} onchange={() => load(true)}>
        <option value={5}>5 miles</option>
        <option value={10}>10 miles</option>
        <option value={20}>20 miles</option>
        <option value={30}>30 miles</option>
      </select>
    </label>
  </div>

  {#if matchID}
    <div class="match-flood stack" role="status">
      <p class="score">It's a match</p>
      <h2>You both said yes.</h2>
      <p>Open the match to see their week and, if they choose, their Instagram.</p>
      <div class="row">
        <a class="button" href={`/matches/${matchID}`}>Open match</a>
        <button class="secondary" onclick={() => (matchID = null)}>Keep looking</button>
      </div>
    </div>
  {/if}

  {#if blockedReason}
    <div class="alert-box stack" role="alert">
      <strong>Discover is not open yet.</strong>
      <p>{blockedReason}</p>
      <a class="button" href="/onboarding">Finish setup</a>
    </div>
  {:else if loading && !cards.length}
    <p role="status">Loading players…</p>
  {:else if error}
    <div class="alert-box stack" role="alert">
      <p>{error}</p>
      <button class="secondary" onclick={() => load(true)}>Try again</button>
    </div>
  {:else if !cards.length}
    <div class="surface stack">
      <h2>No one in range right now</h2>
      <p class="muted">Try a wider radius. Discovery shows players active in the last seven days.</p>
    </div>
  {:else}
    <div class="grid two">
      {#each cards as card (card.player.player_id)}
        <article class="card player-card" aria-labelledby={`name-${card.player.player_id}`}>
          <div class="photo-grid">
            {#if card.player.photo_urls.length}
              {#each card.player.photo_urls as url, index}
                <img class="photo" src={url} alt={`${card.player.display_name}, photo ${index + 1}`} />
              {/each}
            {:else}
              <div class="photo placeholder" aria-hidden="true"></div>
            {/if}
          </div>
          <div class="spread">
            <h2 id={`name-${card.player.player_id}`}>{card.player.display_name}, {card.player.age}</h2>
            <span class="score">{distanceLabel(card.player.distance_miles)}</span>
          </div>
          <p class="score">{card.player.skill_level} · {card.player.play_summary || 'No shared times listed'}</p>
          {#if card.player.bio}<p>{card.player.bio}</p>{/if}
          <div class="row actions">
            <button
              onclick={() => react(card.player.player_id, 'like')}
              disabled={acting === card.player.player_id}
            >
              Like
            </button>
            {#if confirmPass === card.player.player_id}
              <button
                class="danger"
                onclick={() => react(card.player.player_id, 'pass')}
                disabled={acting === card.player.player_id}
              >
                Confirm: not for me
              </button>
              <button class="ghost" onclick={() => (confirmPass = null)}>Cancel</button>
            {:else}
              <button class="secondary" onclick={() => (confirmPass = card.player.player_id)}>
                Not for me
              </button>
            {/if}
            <a class="button secondary" href={`/report?target=${card.player.player_id}`}>Report</a>
          </div>
        </article>
      {/each}
    </div>
    {#if cursor}
      <button class="secondary" onclick={() => load(false)} disabled={loading}>
        {loading ? 'Loading…' : 'More players'}
      </button>
    {/if}
  {/if}
</section>

<style>
  .radius {
    width: 9rem;
  }

  .player-card {
    display: grid;
    gap: 0.75rem;
  }

  .placeholder {
    background: linear-gradient(135deg, #e5eae7, #f2f5f3);
  }

  .actions {
    margin-top: auto;
  }
</style>
