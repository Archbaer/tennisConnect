<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { apiJSON, type Profile } from '#lib/api';
  import AvailabilityForm from '#lib/AvailabilityForm.svelte';

  let profile: Profile | null = null;
  let loading = true;
  let error = '';

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
  <div class="spread">
    <div>
      <p class="score">Your card</p>
      <h1>Play times</h1>
    </div>
    <a href="/profile">Cancel</a>
  </div>
  {#if loading}
    <p role="status">Loading play times…</p>
  {:else if !profile}
    <p class="alert-box" role="alert">{error || 'Play times are unavailable.'}</p>
  {:else}
    <div class="surface">
      <AvailabilityForm
        {profile}
        onSaved={() => {
          goto('/profile');
        }}
      />
    </div>
  {/if}
</section>
