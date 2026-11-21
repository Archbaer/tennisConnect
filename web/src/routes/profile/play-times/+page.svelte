<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { apiJSON, type Profile } from '#lib/api';
  import ProfileForm from '#lib/ProfileForm.svelte';

  let profile: Profile | null = null;
  let loading = true;

  onMount(async () => {
    try {
      profile = await apiJSON<Profile>('/api/v1/profile');
    } catch {
      profile = null;
    }
    loading = false;
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
  <p class="notice">Times are Europe/London. Slots cannot cross midnight.</p>
  {#if loading}
    <p role="status">Loading play times…</p>
  {:else}
    <div class="surface">
      <ProfileForm
        initial={profile}
        submitLabel="Save play times"
        onSaved={() => {
          goto('/profile');
        }}
      />
    </div>
  {/if}
</section>
