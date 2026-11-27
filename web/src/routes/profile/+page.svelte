<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON, type Photo, type Profile, weekdayName } from '#lib/api';
  import { playWindows, type PlayWindow } from '#lib/dashboard';
  import AreaSheet from '#lib/AreaSheet.svelte';
  import PreviewDialog from '#lib/PreviewDialog.svelte';

  let profile: Profile | null = null;
  let photos: Photo[] = [];
  let photoUrls: Record<string, string> = {};
  let area = '';
  let loading = true;
  let error = '';
  let areaSheet: { open: () => Promise<void> };
  let preview: { open: () => Promise<void> };

  const segments = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9];

  let approved = 0;
  let pendingPhotos = 0;
  let windows: { totalHours: number; windows: PlayWindow[] } = { totalHours: 0, windows: [] };
  let completed = 0;
  let completeness = 0;

  $: mainPhoto = photos.find((photo) => photo.status === 'approved' && photoUrls[photo.photo_id]);
  $: mainPhotoUrl = mainPhoto ? photoUrls[mainPhoto.photo_id] : '';
  $: approved = photos.filter((photo) => photo.status === 'approved').length;
  $: pendingPhotos = photos.filter((photo) => photo.status === 'pending').length;
  $: windows = playWindows(profile?.availability ?? []);
  $: checks = [
    Boolean(approved),
    Boolean(profile?.bio.trim()),
    Boolean(profile?.availability.length),
    Boolean(area)
  ];
  $: completed = checks.filter(Boolean).length;
  $: completeness = Math.round((completed / checks.length) * 100);

  onMount(async () => {
    try {
      profile = await apiJSON<Profile>('/api/v1/profile');
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'Profile unavailable.';
    }
    try {
      photos = (await apiJSON<{ photos: Photo[] }>('/api/v1/photos')).photos;
      const urls: Record<string, string> = {};
      for (const photo of photos) {
        if (photo.status === 'rejected') continue;
        try {
          const url = await apiJSON<{ url: string }>(`/api/v1/photos/${photo.photo_id}/url`);
          urls[photo.photo_id] = url.url;
        } catch {
          // A missing owner thumbnail is not a profile failure.
        }
      }
      photoUrls = urls;
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
  <title>You · Tennis Connect</title>
</svelte:head>

<section class="page profile-page stack">
  <header class="page-head">
    <div>
      <p class="kicker">Your card</p>
      <h1>You</h1>
      <p class="muted mb-0">The card other players see, and the one place to change it.</p>
    </div>
    <div class="page-actions">
      <a class="btn btn-outline-secondary" href="/profile/edit">Edit profile</a>
      <button class="btn btn-primary" type="button" onclick={() => preview.open()} disabled={!profile}>Preview as another player</button>
    </div>
  </header>

  {#if loading}
    <p role="status">Loading your profile…</p>
  {:else if !profile}
    <div class="sheet-card stack">
      <p class="alert-box mb-0" role="alert">{error || 'Your profile is not complete.'}</p>
      <a class="btn btn-primary align-self-start" href="/onboarding">Finish onboarding</a>
    </div>
  {:else}
    <section class="kpi-strip" aria-label="Profile trackers">
      <article class="kpi">
        <h2 class="kpi-label">Profile complete</h2>
        <p class="kpi-value">{completeness}<span class="kpi-unit">%</span></p>
        <p class="kpi-delta">{completed} of {checks.length} checks</p>
        <div class="segmented" role="img" aria-label={`Profile ${completeness} percent complete`}>
          {#each segments as segment}
            <span class="segment" class:on={segment < Math.round(completeness / 10)}></span>
          {/each}
        </div>
      </article>
      <article class="kpi">
        <h2 class="kpi-label">Weekly play</h2>
        <p class="kpi-value">{windows.totalHours}<span class="kpi-unit">h</span></p>
        <p class="kpi-delta">{profile.availability.length} saved windows</p>
      </article>
      <article class="kpi">
        <h2 class="kpi-label">Photos approved</h2>
        <p class="kpi-value">{approved}</p>
        <p class="kpi-delta">{pendingPhotos} awaiting review</p>
      </article>
      <article class="kpi">
        <h2 class="kpi-label">Area</h2>
        <p class="kpi-value small">{area ? 'Set' : 'Missing'}</p>
        <p class="kpi-delta">{area || 'Add a coarse area'}</p>
      </article>
    </section>

    {#if !approved}
      <div class="alert alert-warning d-flex flex-wrap align-items-center justify-content-between gap-2" role="status">
        <span>You're not in Discover yet — one approved photo is needed.</span>
        <a class="btn btn-sm btn-primary" href="/profile/photos">Review photos</a>
      </div>
    {/if}

    <article class="media-card">
      <div class="media-frame profile-photo">
        <div class="media-fallback" aria-hidden="true">{initials(profile.display_name)}</div>
        {#if mainPhotoUrl}
          <img src={mainPhotoUrl} alt={`${profile.display_name}'s main photo`} onerror={(event) => ((event.currentTarget as HTMLImageElement).hidden = true)} />
        {/if}
        <span class="chip media-chip">{area || 'Area not set'}</span>
      </div>
      <div class="media-body">
        <div class="name-row">
          <h2 class="mb-0">{profile.display_name}{profile.age ? `, ${profile.age}` : ''}</h2>
          <span class="chip level">{profile.skill_level}</span>
        </div>
        <p class="muted mb-0">
          {profile.gender === 'woman' ? 'Woman' : 'Man'} · interested in {profile.gender_preference} ·
          ages {profile.min_age}–{profile.max_age}
        </p>
        {#if profile.bio}<p class="mb-0">{profile.bio}</p>{/if}
        <div class="page-actions">
          <a class="btn btn-primary" href="/discover">Go to Discover</a>
          <button class="btn btn-outline-secondary" type="button" onclick={() => areaSheet.open()}>Change area</button>
        </div>
      </div>
    </article>

    {#if photos.length}
      <section class="sheet-card stack" aria-labelledby="profile-photos-title">
        <div class="spread">
          <h2 id="profile-photos-title" class="mb-0">Photos</h2>
          <a href="/profile/photos">Manage</a>
        </div>
        <div class="photo-grid">
          {#each photos as photo}
            <div class="photo-slot">
              <div class="photo-placeholder" aria-hidden="true"><i class="bi bi-image"></i></div>
              {#if photoUrls[photo.photo_id]}
                <img
                  src={photoUrls[photo.photo_id]}
                  alt={`Photo ${photo.display_order}`}
                  onerror={(event) => ((event.currentTarget as HTMLImageElement).hidden = true)}
                />
              {/if}
              <span class="photo-status chip" class:success={photo.status === 'approved'} class:danger={photo.status === 'rejected'}>{photo.status === 'approved' ? 'Approved' : photo.status === 'rejected' ? 'Not accepted' : 'Pending'}</span>
            </div>
          {/each}
        </div>
      </section>
    {/if}

    <section class="sheet-card stack" aria-labelledby="profile-play-title">
      <div class="spread">
        <h2 id="profile-play-title" class="mb-0">Play times</h2>
        <a href="/profile/play-times">Edit</a>
      </div>
      <ul class="plain-list">
        {#each profile.availability as slot}
          <li>{weekdayName(slot.weekday)}, {slot.start}–{slot.end}</li>
        {/each}
      </ul>
    </section>

    <section class="sheet-card stack" aria-labelledby="profile-area-title">
      <div class="spread">
        <h2 id="profile-area-title" class="mb-0">Area</h2>
        <button class="link-button" type="button" onclick={() => areaSheet.open()}>Change</button>
      </div>
      <p class="mb-0">{area || 'No area set yet.'}</p>
    </section>

    <section class="sheet-card stack" aria-labelledby="profile-settings-title">
      <div class="spread">
        <h2 id="profile-settings-title" class="mb-0">Settings</h2>
        <a href="/settings">Open</a>
      </div>
      <p class="muted mb-0">Matching, privacy, export and account controls.</p>
    </section>

    <AreaSheet bind:this={areaSheet} onSaved={(label) => (area = label)} />
    <PreviewDialog bind:this={preview} profile={profile} photoUrl={mainPhotoUrl} {area} />
  {/if}
</section>

<style>
  .profile-page {
    width: min(100% - 1.5rem, 46rem);
  }

  .level {
    text-transform: capitalize;
  }

  .photo-status {
    position: absolute;
    left: 0.4rem;
    bottom: 0.4rem;
    min-height: 1.35rem;
    padding: 0.05rem 0.4rem;
    font-size: 0.62rem;
    box-shadow: 0 2px 8px rgb(0 0 0 / 12%), inset 0 1px 0 rgb(255 255 255 / 40%);
  }

  .link-button {
    min-height: 0;
    padding: 0;
    border: 0;
    border-radius: 0;
    background: transparent;
    color: var(--court);
    font: inherit;
    font-weight: 600;
    text-decoration: underline;
    text-underline-offset: 0.18em;
  }

  .link-button:hover {
    background: transparent;
    color: var(--court-deep);
    transform: none;
    box-shadow: none;
  }
</style>
