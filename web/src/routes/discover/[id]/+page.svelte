<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import {
    ApiError,
    apiJSON,
    distanceLabel,
    weekdayLabel,
    type DiscoveryDetail
  } from '#lib/api';
  import { noteMatchCreated } from '#lib/matchState';
  import { online } from '#lib/offline';
  import SafetyDialog from '#lib/SafetyDialog.svelte';

  let detail: DiscoveryDetail | null = null;
  let targetID = '';
  let radius = 10;
  let loading = true;
  let error = '';
  let acting = false;
  let confirmPass = false;
  let safety: { openReport: () => Promise<void>; openBlock: () => Promise<void> };

  onMount(() => {
    const url = new URL(window.location.href);
    targetID = url.pathname.split('/').filter(Boolean).pop() ?? '';
    const value = Number(url.searchParams.get('radius'));
    radius = [5, 10, 20, 30].includes(value) ? value : 10;
    load();
  });

  async function load() {
    loading = true;
    error = '';
    try {
      detail = await apiJSON<DiscoveryDetail>(`/api/v1/discovery/${targetID}?radius=${radius}`);
    } catch (problem) {
      if (problem instanceof ApiError && problem.status === 404) {
        error = 'That player is no longer available.';
      } else {
        error = problem instanceof Error ? problem.message : 'That profile is unavailable.';
      }
    } finally {
      loading = false;
    }
  }

  async function react(action: 'like' | 'pass') {
    if (!detail) return;
    acting = true;
    error = '';
    try {
      const result = await apiJSON<{ matched: boolean; match_id?: string }>('/api/v1/reactions', {
        method: 'POST',
        body: JSON.stringify({ target_id: detail.player.player_id, action })
      });
      if (result.matched && result.match_id) {
        noteMatchCreated();
        await goto(`/matches/${result.match_id}/new`);
      } else {
        await goto('/discover');
      }
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'That reaction did not save.';
    } finally {
      acting = false;
    }
  }

  async function openSafety(mode: 'report' | 'block') {
    if (mode === 'block') {
      await safety.openBlock();
    } else {
      await safety.openReport();
    }
  }

  function initials(name: string): string {
    return name
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((part) => part[0]?.toUpperCase() ?? '')
      .join('');
  }
</script>

<svelte:head>
  <title>{detail ? `${detail.player.display_name} · Tennis Connect` : 'Player · Tennis Connect'}</title>
</svelte:head>

<section class="page detail-page stack">
  <a class="back" href="/discover"><i class="bi bi-arrow-left" aria-hidden="true"></i> Discover</a>

  {#if loading}
    <p role="status">Loading player…</p>
  {:else if !detail}
    <div class="surface stack">
      <p class="alert-box mb-0" role="alert">{error}</p>
      <a class="btn btn-primary align-self-start" href="/discover">Back to Discover</a>
    </div>
  {:else}
    <article class="player-card" aria-labelledby="detail-name">
      <div class="photo-wrap">
        <div class="photo-fallback" aria-hidden="true"><span>{initials(detail.player.display_name)}</span></div>
        {#if detail.player.photo_urls.length}
          <img
            src={detail.player.photo_urls[0]}
            alt={`${detail.player.display_name}'s photo`}
            onerror={(event) => ((event.currentTarget as HTMLImageElement).style.display = 'none')}
          />
        {/if}
        <span class="chip photo-chip">{distanceLabel(detail.player.distance_miles)}</span>
        <button class="icon-button card-safety" type="button" onclick={() => openSafety('report')} aria-label={`Safety options for ${detail.player.display_name}`}>
          <i class="bi bi-flag" aria-hidden="true"></i>
        </button>
      </div>

      <div class="card-body">
        <div class="name-row">
          <h1 id="detail-name" class="mb-0">{detail.player.display_name}, {detail.player.age}</h1>
          <span class="chip">{detail.player.skill_level}</span>
        </div>

        <div class="week-strip" aria-hidden="true">
          {#each [1, 2, 3, 4, 5, 6, 7] as day}
            <span class:active={detail.player.availability.some((slot) => slot.weekday === day)}>{weekdayLabel(day)[0]}</span>
          {/each}
        </div>

        {#if detail.player.bio}<p class="bio">{detail.player.bio}</p>{/if}

        <section class="stack gap-2" aria-labelledby="detail-times">
          <h2 id="detail-times" class="mb-0">Play times</h2>
          <ul class="times">
            {#each detail.player.availability as slot}
              <li>{weekdayLabel(slot.weekday)} {slot.start}–{slot.end}</li>
            {/each}
          </ul>
        </section>

        {#if error}
          <p class="alert alert-danger py-2 mb-0" role="alert">{error}</p>
        {/if}

        {#if confirmPass}
          <div class="confirm-pass">
            <p class="small mb-2">You won't see each other again.</p>
            <div class="d-flex gap-2">
              <button class="btn btn-danger flex-grow-1" onclick={() => react('pass')} disabled={acting || !$online}>Not for me</button>
              <button class="btn btn-outline-secondary" onclick={() => (confirmPass = false)}>Cancel</button>
            </div>
          </div>
        {:else}
          <div class="card-actions">
            <button class="btn btn-primary" onclick={() => react('like')} disabled={acting || !$online}>
              {acting ? 'Sending…' : "I'd play"}
            </button>
            <button class="btn btn-outline-secondary" onclick={() => (confirmPass = true)} disabled={!$online}>
              Not for me
            </button>
          </div>
        {/if}
      </div>
    </article>

    <SafetyDialog
      bind:this={safety}
      targetId={detail.player.player_id}
      targetName={detail.player.display_name}
      onBlocked={() => goto('/discover')}
      onReported={() => goto('/discover')}
    />
  {/if}
</section>

<style>
  .detail-page {
    width: min(100% - 1.5rem, 30rem);
    padding-block: 1rem 1.5rem;
  }

  .back {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-weight: 700;
    text-decoration: none;
  }

  .player-card {
    overflow: hidden;
    border: 1px solid var(--line);
    border-radius: var(--radius);
    background: var(--surface);
    box-shadow: var(--shadow);
  }

  .photo-wrap {
    position: relative;
    aspect-ratio: 4 / 3;
    max-height: 20rem;
    overflow: hidden;
    background: #dfe9df;
  }

  .photo-wrap img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .photo-fallback {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    background:
      linear-gradient(135deg, rgb(216 255 62 / 18%), transparent 55%),
      linear-gradient(160deg, var(--court-light), var(--court-deep));
    color: var(--ball);
    font-size: clamp(4rem, 18vw, 7rem);
    font-weight: 800;
  }

  .photo-fallback::before {
    content: '';
    position: absolute;
    inset: 8%;
    border: 2px solid rgb(247 249 244 / 32%);
    border-radius: 50%;
    transform: rotate(-12deg);
  }

  .photo-chip {
    position: absolute;
    left: 0.75rem;
    top: 0.75rem;
    background: rgb(255 255 255 / 92%);
  }

  .card-safety {
    position: absolute;
    right: 0.75rem;
    top: 0.75rem;
  }

  .card-body {
    display: grid;
    gap: 0.75rem;
    padding: 1rem;
  }

  .name-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
  }

  .name-row h1 {
    font-size: clamp(1.6rem, 6vw, 2rem);
  }

  .week-strip {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    gap: 0.3rem;
  }

  .week-strip span {
    display: grid;
    place-items: center;
    min-height: 1.75rem;
    border: 1px solid var(--line);
    border-radius: var(--radius-chip);
    color: var(--muted);
    font-size: 0.72rem;
    font-weight: 700;
  }

  .week-strip span.active {
    border-color: var(--court);
    background: var(--court);
    color: white;
  }

  .bio {
    white-space: pre-wrap;
  }

  .times {
    margin: 0;
    padding-left: 1.2rem;
  }

  .card-actions {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.6rem;
  }

  .card-actions .btn {
    min-height: 2.8rem;
  }

  .confirm-pass {
    padding: 0.75rem;
    border: 1px solid #e6c4bf;
    border-radius: var(--radius-control);
    background: #fbefeb;
  }

  @media (max-width: 360px) {
    .card-actions {
      grid-template-columns: 1fr;
    }
  }
</style>
