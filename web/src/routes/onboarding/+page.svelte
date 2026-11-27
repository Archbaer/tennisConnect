<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { apiJSON, type Photo, type Profile } from '#lib/api';
  import AreaEditor from '#lib/AreaEditor.svelte';
  import ProfileForm from '#lib/ProfileForm.svelte';
  import PhotoManager from '#lib/PhotoManager.svelte';

  let profile: Profile | null = null;
  let photos: Photo[] = [];
  let areaLabel = '';
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

  async function finish() {
    await goto('/discover');
  }
</script>

<svelte:head>
  <title>Set up your player card · Tennis Connect</title>
</svelte:head>

<section class="page narrow stack">
  <header class="page-head">
    <div>
      <p class="kicker">Onboarding · {step} of 3</p>
      <h1>Build your player card</h1>
    </div>
  </header>

  <div class="onboarding-tracker">
    <div class="segmented" role="progressbar" aria-label="Onboarding progress" aria-valuenow={step} aria-valuemin="1" aria-valuemax="3">
      {#each [1, 2, 3] as marker}
        <span class="segment" class:on={step >= marker}></span>
      {/each}
    </div>
    <p class="muted mb-0">Only what another player needs to decide whether to play with you.</p>
  </div>

  {#if step === 1}
    <div class="sheet-card stack">
      <h2><i class="bi bi-person me-2" aria-hidden="true"></i>Your profile</h2>
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
    <div class="sheet-card stack">
      <h2><i class="bi bi-geo-alt me-2" aria-hidden="true"></i>Where you play</h2>
      <AreaEditor
        initialArea={areaLabel}
        onSaved={(label) => {
          areaLabel = label;
          step = 3;
        }}
      />
      <button
        class="btn btn-outline-secondary"
        type="button"
        onclick={() => {
          step = 3;
        }}>Skip for now</button
      >
    </div>
  {:else}
    <div class="sheet-card stack">
      <h2><i class="bi bi-flag me-2" aria-hidden="true"></i>Ready check</h2>
      <ul class="checklist">
        <li><span>Profile</span><strong>{profile ? 'Saved' : 'Missing'}</strong></li>
        <li><span>Approved photo</span><strong>{photos.filter((photo) => photo.status === 'approved').length} approved</strong></li>
        <li><span>Photos waiting</span><strong>{photos.filter((photo) => photo.status === 'pending').length} not confirmed</strong></li>
        <li><span>Area</span><strong>{areaLabel || 'Not set'}</strong></li>
      </ul>
      <p class="notice">
        Discover needs a profile, an area and one approved photo. Pending photos are visible to nobody.
      </p>
      {#if ready}
        <button class="btn btn-primary" onclick={finish}>Go to Discover</button>
      {:else}
        <div class="d-flex flex-wrap gap-2">
          <button class="btn btn-outline-secondary" onclick={() => (step = 1)}>Review profile</button>
          <button class="btn btn-outline-secondary" onclick={() => (step = 2)}>Review photos and area</button>
        </div>
      {/if}
    </div>
  {/if}
</section>

<style>
  .onboarding-tracker {
    display: grid;
    gap: 0.4rem;
  }

  .onboarding-tracker .segmented {
    height: 8px;
  }

  .checklist {
    display: grid;
    gap: 0;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .checklist li {
    display: flex;
    justify-content: space-between;
    gap: 1rem;
    padding-block: 0.55rem;
    border-bottom: 1px solid var(--line);
  }

  .checklist li:last-child {
    border-bottom: 0;
  }
</style>
