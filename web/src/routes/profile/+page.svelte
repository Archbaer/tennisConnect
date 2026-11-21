<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON, type Photo, type Profile } from '#lib/api';

  let profile: Profile | null = null;
  let photos: Photo[] = [];
  let area = '';
  let loading = true;
  let error = '';

  onMount(async () => {
    try {
      profile = await apiJSON<Profile>('/api/v1/profile');
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'Profile unavailable.';
    }
    try {
      photos = (await apiJSON<{ photos: Photo[] }>('/api/v1/photos')).photos;
    } catch {
      photos = [];
    }
    try {
      area = (await apiJSON<{ area_label: string }>('/api/v1/location')).area_label;
    } catch {
      area = '';
    }
    loading = false;
  });
</script>

<svelte:head>
  <title>You · Tennis Connect</title>
</svelte:head>

<section class="page narrow stack">
  <div class="spread">
    <div>
      <p class="score">Your card</p>
      <h1>You</h1>
    </div>
    <a class="button secondary" href="/profile/edit">Edit profile</a>
  </div>

  {#if loading}
    <p role="status">Loading your profile…</p>
  {:else if !profile}
    <div class="surface stack">
      <p class="alert-box" role="alert">{error || 'Your profile is not complete.'}</p>
      <a class="button" href="/onboarding">Finish onboarding</a>
    </div>
  {:else}
    <div class="surface stack">
      <div class="spread">
        <h2>{profile.display_name}{profile.age ? `, ${profile.age}` : ''}</h2>
        <span class="chip">{profile.skill_level}</span>
      </div>
      {#if profile.bio}<p>{profile.bio}</p>{/if}
      <p class="muted">
        {profile.gender === 'woman' ? 'Woman' : 'Man'} · interested in {profile.gender_preference} ·
        ages {profile.min_age}–{profile.max_age}
      </p>
      {#if profile.instagram_handle}
        <p class="muted">Instagram is shared only after a mutual match.</p>
      {/if}
    </div>

    <div class="surface stack">
      <div class="spread">
        <h2>Play times</h2>
        <a href="/profile/play-times">Edit</a>
      </div>
      <ul>
        {#each profile.availability as slot}
          <li>Day {slot.weekday}, {slot.start}–{slot.end}</li>
        {/each}
      </ul>
    </div>

    <div class="surface stack">
      <div class="spread">
        <h2>Photos</h2>
        <a href="/profile/photos">Manage</a>
      </div>
      <p>{photos.length} of 3 slots used.</p>
      <ul>
        {#each photos as photo}
          <li>Photo {photo.display_order}: {photo.status}</li>
        {/each}
      </ul>
    </div>

    <div class="surface stack">
      <div class="spread">
        <h2>Area</h2>
        <a href="/onboarding">Change</a>
      </div>
      <p>{area || 'No area set yet.'}</p>
    </div>

    <div class="surface stack">
      <h2>Settings</h2>
      <a href="/settings">Matching, privacy and account</a>
    </div>
  {/if}
</section>
