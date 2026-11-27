<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import {
    ApiError,
    apiJSON,
    distanceLabel,
    weekdayLabel,
    weekdayName,
    type DiscoveryDetail
  } from '#lib/api';
  import { playWindows, type PlayWindow } from '#lib/dashboard';
  import LoadingSkeleton from '#lib/LoadingSkeleton.svelte';
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
  let windows: { totalHours: number; windows: PlayWindow[] } = { totalHours: 0, windows: [] };
  let safety: { openReport: () => Promise<void>; openBlock: () => Promise<void> };

  const segments = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9];

  $: windows = detail ? playWindows(detail.player.availability) : { totalHours: 0, windows: [] };

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
    <LoadingSkeleton variant="player" />
  {:else if !detail}
    <div class="sheet-card stack">
      <p class="alert-box mb-0" role="alert">{error}</p>
      <a class="btn btn-primary align-self-start" href="/discover">Back to Discover</a>
    </div>
  {:else}
    <article class="media-card" aria-labelledby="detail-name">
      <div class="media-frame">
        <div class="media-fallback" aria-hidden="true"><span>{initials(detail.player.display_name)}</span></div>
        {#if detail.player.photo_urls.length}
          <img
            src={detail.player.photo_urls[0]}
            alt={`${detail.player.display_name}'s photo`}
            onerror={(event) => ((event.currentTarget as HTMLImageElement).hidden = true)}
          />
        {/if}
        <span class="chip media-chip">{distanceLabel(detail.player.distance_miles)}</span>
        <button class="icon-button card-safety" type="button" onclick={() => openSafety('report')} aria-label={`Safety options for ${detail.player.display_name}`}>
          <i class="bi bi-flag" aria-hidden="true"></i>
        </button>
      </div>

      <div class="media-body">
        <div class="name-row">
          <h1 id="detail-name" class="mb-0">{detail.player.display_name}, {detail.player.age}</h1>
          <span class="chip level">{detail.player.skill_level}</span>
        </div>

        <div class="week-strip" aria-hidden="true">
          {#each [1, 2, 3, 4, 5, 6, 7] as day}
            <span class:active={detail.player.availability.some((slot) => slot.weekday === day)}>{weekdayLabel(day)[0]}</span>
          {/each}
        </div>

        {#if detail.player.bio}<p class="bio">{detail.player.bio}</p>{/if}

        <section class="kpi-strip three" aria-label="Player stats">
          <article class="kpi">
            <h2 class="kpi-label">Level</h2>
            <p class="kpi-value small level">{detail.player.skill_level}</p>
            <p class="kpi-delta">Self-declared</p>
          </article>
          <article class="kpi">
            <h2 class="kpi-label">Distance</h2>
            <p class="kpi-value small">{distanceLabel(detail.player.distance_miles)}</p>
            <p class="kpi-delta">Coarse bucket</p>
          </article>
          <article class="kpi">
            <h2 class="kpi-label">Weekly play</h2>
            <p class="kpi-value">{windows.totalHours}<span class="kpi-unit">h</span></p>
            <p class="kpi-delta">{detail.player.availability.length} windows</p>
          </article>
        </section>

        <section class="stack gap-2" aria-labelledby="detail-times">
          <h2 id="detail-times" class="mb-0">Play times</h2>
          <ul class="plain-list times">
            {#each detail.player.availability as slot}
              <li>{weekdayName(slot.weekday)}, {slot.start}–{slot.end}</li>
            {/each}
          </ul>

          {#if windows.windows.length}
            <ul class="progress-list">
              {#each windows.windows as slot}
                <li>
                  <div class="progress-row">
                    <span>{slot.label}</span>
                    <span class="progress-value">{slot.hours}h</span>
                  </div>
                  <div class="segmented" role="img" aria-label={`${slot.label}: ${slot.hours} hours`}>
                    {#each segments as segment}
                      <span class="segment" class:on={segment < Math.round(slot.percent / 10)}></span>
                    {/each}
                  </div>
                </li>
              {/each}
            </ul>
          {/if}
        </section>

        {#if error}
          <p class="alert alert-danger py-2 mb-0" role="alert">{error}</p>
        {/if}

        {#if confirmPass}
          <div class="confirm-pass sticky-actions">
            <p class="small mb-2">You won't see each other again.</p>
            <div class="d-flex gap-2">
              <button class="btn btn-danger flex-grow-1" onclick={() => react('pass')} disabled={acting || !$online}>Not for me</button>
              <button class="btn btn-outline-secondary" onclick={() => (confirmPass = false)}>Cancel</button>
            </div>
          </div>
        {:else}
          <div class="card-actions sticky-actions">
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
    width: min(100% - (2 * var(--space-page-x)), 30rem);
    padding-block: var(--space-page-top) var(--space-page-bottom);
  }

  .back {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-weight: 700;
    text-decoration: none;
  }

  .level {
    text-transform: capitalize;
  }

  .times {
    margin-bottom: 0;
  }

  .media-body h1 {
    font-size: clamp(1.6rem, 6vw, 2rem);
  }

  @media (max-width: 360px) {
    .card-actions {
      grid-template-columns: 1fr;
    }
  }
</style>
