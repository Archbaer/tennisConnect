<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { apiJSON, type Profile } from '#lib/api';
  import { playWindows, type PlayWindow } from '#lib/dashboard';
  import AvailabilityForm from '#lib/AvailabilityForm.svelte';

  let profile: Profile | null = null;
  let loading = true;
  let error = '';
  let windows: { totalHours: number; windows: PlayWindow[] } = { totalHours: 0, windows: [] };

  $: windows = playWindows(profile?.availability ?? []);

  onMount(async () => {
    try {
      profile = await apiJSON<Profile>('/api/v1/profile');
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'Play times are unavailable.';
    } finally {
      loading = false;
    }
  });
</script>

<svelte:head>
  <title>Play times · Tennis Connect</title>
</svelte:head>

<section class="page narrow stack">
  <header class="page-head">
    <div>
      <p class="kicker">Your card</p>
      <h1>Play times</h1>
    </div>
    <a href="/profile">Cancel</a>
  </header>

  {#if loading}
    <p role="status">Loading play times…</p>
  {:else if !profile}
    <p class="alert-box" role="alert">{error || 'Play times are unavailable.'}</p>
  {:else}
    <section class="kpi-strip three" aria-label="Play-time summary">
      <article class="kpi">
        <h2 class="kpi-label">Weekly hours</h2>
        <p class="kpi-value">{windows.totalHours}<span class="kpi-unit">h</span></p>
        <p class="kpi-delta">Available to play</p>
      </article>
      <article class="kpi">
        <h2 class="kpi-label">Saved windows</h2>
        <p class="kpi-value">{profile.availability.length}</p>
        <p class="kpi-delta">Inside one day each</p>
      </article>
      <article class="kpi">
        <h2 class="kpi-label">Time zone</h2>
        <p class="kpi-value small">London</p>
        <p class="kpi-delta">Europe/London time</p>
      </article>
    </section>
    <div class="sheet-card">
      <AvailabilityForm
        {profile}
        onSaved={() => {
          goto('/profile');
        }}
      />
    </div>
  {/if}
</section>
