<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { apiJSON, type Photo, type Profile } from '#lib/api';
  import ProfileForm from '#lib/ProfileForm.svelte';
  import PhotoManager from '#lib/PhotoManager.svelte';

  let profile: Profile | null = null;
  let photos: Photo[] = [];
  let area = '';
  let areaLabel = '';
  let areaError = '';
  let step = 1;
  let ready = false;

  onMount(async () => {
    try {
      profile = await apiJSON<Profile>('/api/v1/profile');
      step = 2;
    } catch {
      profile = null;
    }
    try {
      photos = (await apiJSON<{ photos: Photo[] }>('/api/v1/photos')).photos;
    } catch {
      photos = [];
    }
    try {
      const location = await apiJSON<{ area_label: string }>('/api/v1/location');
      areaLabel = location.area_label;
    } catch {
      areaLabel = '';
    }
    ready = Boolean(profile && photos.some((photo) => photo.status === 'approved') && areaLabel);
  });

  async function refreshPhotos() {
    try {
      photos = (await apiJSON<{ photos: Photo[] }>('/api/v1/photos')).photos;
      ready = Boolean(profile && photos.some((photo) => photo.status === 'approved') && areaLabel);
    } catch {
      photos = [];
    }
  }

  async function saveArea(event: SubmitEvent) {
    event.preventDefault();
    areaError = '';
    try {
      const result = await apiJSON<{ area_label: string }>('/api/v1/location', {
        method: 'PUT',
        body: JSON.stringify({ area })
      });
      areaLabel = result.area_label;
      step = 3;
    } catch (problem) {
      areaError = problem instanceof Error ? problem.message : 'We could not find that area.';
    }
  }

  async function finish() {
    await goto('/discover');
  }
</script>

<svelte:head>
  <title>Set up your player card · Tennis Connect</title>
</svelte:head>

<section class="page narrow stack">
  <div>
    <p class="score">Onboarding · {step} of 3</p>
    <h1>Build your player card</h1>
    <p class="muted">Only what another player needs to decide whether to play with you.</p>
  </div>

  {#if step === 1}
    <div class="surface stack">
      <h2>Your profile</h2>
      <ProfileForm
        initial={profile}
        submitLabel="Save and continue"
        onSaved={() => {
          step = 2;
        }}
      />
    </div>
  {:else if step === 2}
    <PhotoManager {photos} onChange={refreshPhotos} />
    <div class="surface stack">
      <h2>Where you play</h2>
      <p class="muted">A postcode or neighbourhood. We store a coarse area and never return coordinates.</p>
      {#if areaLabel}
        <p class="notice">Your area: {areaLabel}</p>
      {/if}
      {#if areaError}
        <p class="alert-box" role="alert">{areaError}</p>
      {/if}
      <form class="row" onsubmit={saveArea}>
        <label class="grow">
          Postcode or area
          <input required bind:value={area} placeholder="SW11 4NJ or Battersea" />
        </label>
        <button type="submit">Save area</button>
      </form>
      <button
        class="secondary"
        type="button"
        onclick={() => {
          step = 3;
        }}>Skip for now</button
      >
    </div>
  {:else}
    <div class="surface stack">
      <h2>Ready check</h2>
      <ul>
        <li>Profile: {profile ? 'saved' : 'missing'}</li>
        <li>Approved photo: {photos.some((photo) => photo.status === 'approved') ? 'yes' : 'not yet'}</li>
        <li>Area: {areaLabel || 'not set'}</li>
      </ul>
      <p class="notice">
        Discover needs a profile, an area and one approved photo. Photo review is synchronous.
      </p>
      {#if ready}
        <button onclick={finish}>Go to Discover</button>
      {:else}
        <button class="secondary" onclick={() => (step = 1)}>Review my setup</button>
      {/if}
    </div>
  {/if}
</section>

<style>
  .grow {
    flex: 1;
  }
</style>
