<script lang="ts">
  import { onMount } from 'svelte';
  import { ApiError, apiJSON, distanceLabel, type DiscoveryCard } from '#lib/api';
  import { online } from '#lib/offline';

  let cards: DiscoveryCard[] = [];
  let cursor: string | null = null;
  let radius = 10;
  let loading = true;
  let error = '';
  let blockedReason = '';
  let matchID: string | null = null;
  let confirmPass: string | null = null;
  let acting: string | null = null;

  $: card = cards[0];

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
      cards = cards.filter((item) => item.player.player_id !== targetID);
      confirmPass = null;
      if (result.matched && result.match_id) {
        matchID = result.match_id;
      }
      if (!cards.length && cursor) await load(false);
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
  <div class="d-flex flex-wrap justify-content-between align-items-end gap-3">
    <div>
      <p class="score mb-1"><span class="tennis-ball me-2" aria-hidden="true"></span>Discover</p>
      <h1 class="mb-0">Players near you</h1>
    </div>
    <label class="radius">
      <span class="score">Radius</span>
      <select class="form-select" bind:value={radius} onchange={() => load(true)}>
        <option value={5}>5 miles</option>
        <option value={10}>10 miles</option>
        <option value={20}>20 miles</option>
        <option value={30}>30 miles</option>
      </select>
    </label>
  </div>

  {#if matchID}
    <div class="match-flood stack" role="status">
      <p class="score mb-0">It's a match</p>
      <h2 class="display-6">You're both in.</h2>
      <p>Open the match to see their week and, if they choose, their Instagram.</p>
      <div class="d-flex flex-wrap gap-2">
        <a class="btn btn-primary" href={`/matches/${matchID}`}>Open match</a>
        <button class="btn btn-outline-light" onclick={() => (matchID = null)}>Keep looking</button>
      </div>
    </div>
  {/if}

  {#if blockedReason}
    <div class="alert alert-warning stack" role="alert">
      <strong>Discover is not open yet.</strong>
      <p class="mb-0">{blockedReason}</p>
      <a class="btn btn-primary align-self-start" href="/onboarding">Finish setup</a>
    </div>
  {:else if loading && !cards.length}
    <div class="card p-3" aria-hidden="true">
      <div class="placeholder-glow">
        <div class="placeholder col-12 mb-3" style="aspect-ratio: 4 / 5; border-radius: 14px;"></div>
        <p class="placeholder col-7"></p>
        <p class="placeholder col-4"></p>
      </div>
    </div>
    <p class="visually-hidden" role="status">Loading players…</p>
  {:else if error}
    <div class="alert alert-danger stack" role="alert">
      <p class="mb-0">{error}</p>
      <button class="btn btn-outline-danger align-self-start" onclick={() => load(true)}>Try again</button>
    </div>
  {:else if !card}
    <div class="surface stack text-center">
      <div class="empty-ball mx-auto" aria-hidden="true"></div>
      <h2>No one in range right now</h2>
      <p class="muted">Try a wider radius, add play times, or check your area.</p>
      <div class="d-flex justify-content-center gap-2">
        <button class="btn btn-primary" onclick={() => { radius = 30; load(true); }}>Widen radius</button>
        <a class="btn btn-outline-secondary" href="/onboarding">Change area</a>
      </div>
    </div>
  {:else}
    <article class="card player-card" aria-labelledby={`name-${card.player.player_id}`}>
      <div class="row g-0">
        <div class="col-md-6">
          {#if card.player.photo_urls.length}
            <img class="photo card-photo" src={card.player.photo_urls[0]} alt={`${card.player.display_name}`} />
          {:else}
            <div class="photo card-photo placeholder" aria-hidden="true"></div>
          {/if}
        </div>
        <div class="col-md-6">
          <div class="card-body d-flex flex-column h-100">
            <div class="d-flex justify-content-between align-items-start gap-2">
              <div>
                <h2 id={`name-${card.player.player_id}`} class="mb-1">{card.player.display_name}, {card.player.age}</h2>
                <p class="score text-muted mb-0">{card.player.skill_level} · {distanceLabel(card.player.distance_miles)}</p>
              </div>
              <span class="tennis-ball" aria-hidden="true"></span>
            </div>

            <div class="week-strip my-3" aria-hidden="true">
              {#each ['M', 'T', 'W', 'T', 'F', 'S', 'S'] as day, index}
                <span class:active={card.player.play_summary.toLowerCase().includes(['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'][index])}>{day}</span>
              {/each}
            </div>

            <p class="score mb-1">Plays</p>
            <p>{card.player.play_summary || 'No shared times listed'}</p>
            {#if card.player.bio}<p class="muted">{card.player.bio}</p>{/if}

            {#if error}
              <p class="alert alert-danger py-2" role="alert">{error}</p>
            {/if}

            <div class="mt-auto d-flex flex-wrap gap-2">
              <button
                class="btn btn-primary flex-grow-1"
                onclick={() => react(card.player.player_id, 'like')}
                disabled={acting === card.player.player_id || !$online}
              >
                {acting === card.player.player_id ? 'Sending…' : "I'd play"}
              </button>
              {#if confirmPass === card.player.player_id}
                <div class="confirm-pass w-100">
                  <p class="small mb-2">You won't see each other again.</p>
                  <div class="d-flex gap-2">
                    <button class="btn btn-danger" onclick={() => react(card.player.player_id, 'pass')} disabled={acting === card.player.player_id}>
                      Not for me
                    </button>
                    <button class="btn btn-outline-secondary" onclick={() => (confirmPass = null)}>Cancel</button>
                  </div>
                </div>
              {:else}
                <button class="btn btn-outline-secondary flex-grow-1" onclick={() => (confirmPass = card.player.player_id)} disabled={!$online}>
                  Not for me
                </button>
              {/if}
              <a class="btn btn-outline-secondary" href={`/report?target=${card.player.player_id}`} aria-label={`Report ${card.player.display_name}`}>
                <i class="bi bi-flag" aria-hidden="true"></i>
              </a>
            </div>
          </div>
        </div>
      </div>
    </article>
    {#if cursor}
      <button class="btn btn-outline-secondary" onclick={() => load(false)} disabled={loading}>
        {loading ? 'Loading…' : 'More players'}
      </button>
    {/if}
  {/if}
</section>

<style>
  .radius {
    width: 9rem;
  }

  .card-photo {
    height: 100%;
    min-height: 20rem;
    aspect-ratio: auto;
    border-radius: 16px 0 0 16px;
  }

  .player-card {
    overflow: hidden;
  }

  .week-strip {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    gap: 0.25rem;
    max-width: 15rem;
  }

  .week-strip span {
    display: grid;
    place-items: center;
    aspect-ratio: 1;
    border-radius: 50%;
    border: 1px solid var(--line);
    font-size: 0.7rem;
    font-weight: 700;
    color: var(--muted);
  }

  .week-strip span.active {
    background: var(--ball);
    border-color: var(--ball);
    color: var(--ink);
  }

  .confirm-pass {
    border: 1px solid var(--line);
    border-radius: 12px;
    padding: 0.75rem;
    background: var(--paper);
  }

  .empty-ball {
    width: 3.5rem;
    aspect-ratio: 1;
    border-radius: 50%;
    background: radial-gradient(circle at 35% 30%, #f3ffb1, var(--ball) 55%, var(--ball-deep));
  }

  @media (max-width: 767px) {
    .card-photo {
      min-height: 18rem;
      border-radius: 16px 16px 0 0;
    }
  }
</style>
