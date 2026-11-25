<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON, type Photo, type Profile } from '#lib/api';
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

  $: mainPhoto = photos.find((photo) => photo.status === 'approved' && photoUrls[photo.photo_id]);
  $: mainPhotoUrl = mainPhoto ? photoUrls[mainPhoto.photo_id] : '';

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
  <header class="profile-head">
    <div>
      <p class="score mb-1"><span class="tennis-ball me-2" aria-hidden="true"></span>Your card</p>
      <h1>You</h1>
      <p class="muted mb-0">The card other players see, and the one place to change it.</p>
    </div>
    <div class="profile-actions">
      <a class="btn btn-outline-secondary" href="/profile/edit">Edit profile</a>
      <button class="btn btn-primary" type="button" onclick={() => preview.open()} disabled={!profile}>Preview as another player</button>
    </div>
  </header>

  {#if loading}
    <p role="status">Loading your profile…</p>
  {:else if !profile}
    <div class="surface stack">
      <p class="alert-box mb-0" role="alert">{error || 'Your profile is not complete.'}</p>
      <a class="btn btn-primary align-self-start" href="/onboarding">Finish onboarding</a>
    </div>
  {:else}
    {#if !photos.some((photo) => photo.status === 'approved')}
      <div class="alert alert-warning d-flex flex-wrap align-items-center justify-content-between gap-2" role="status">
        <span>You're not in Discover yet — one approved photo is needed.</span>
        <a class="btn btn-sm btn-primary" href="/profile/photos">Review photos</a>
      </div>
    {/if}

    <article class="profile-hero">
      <div class="hero-photo">
        <div class="hero-fallback" aria-hidden="true">{initials(profile.display_name)}</div>
        {#if mainPhotoUrl}
          <img src={mainPhotoUrl} alt={`${profile.display_name}'s main photo`} onerror={(event) => ((event.currentTarget as HTMLImageElement).style.display = 'none')} />
        {/if}
        <span class="chip hero-chip">{area || 'Area not set'}</span>
      </div>
      <div class="hero-body">
        <div class="name-row">
          <h2 class="mb-0">{profile.display_name}{profile.age ? `, ${profile.age}` : ''}</h2>
          <span class="chip">{profile.skill_level}</span>
        </div>
        <p class="muted mb-0">
          {profile.gender === 'woman' ? 'Woman' : 'Man'} · interested in {profile.gender_preference} ·
          ages {profile.min_age}–{profile.max_age}
        </p>
        {#if profile.bio}<p class="mb-0">{profile.bio}</p>{/if}
        <div class="hero-actions">
          <a class="btn btn-primary" href="/discover">Go to Discover</a>
          <button class="btn btn-outline-secondary" type="button" onclick={() => areaSheet.open()}>Change area</button>
        </div>
      </div>
    </article>

    {#if photos.length}
      <section class="surface stack" aria-labelledby="profile-photos-title">
        <div class="spread">
          <h2 id="profile-photos-title">Photos</h2>
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
                  onerror={(event) => ((event.currentTarget as HTMLImageElement).style.display = 'none')}
                />
              {/if}
            </div>
          {/each}
        </div>
      </section>
    {/if}

    <section class="surface stack" aria-labelledby="profile-play-title">
      <div class="spread">
        <h2 id="profile-play-title">Play times</h2>
        <a href="/profile/play-times">Edit</a>
      </div>
      <ul class="plain-list">
        {#each profile.availability as slot}
          <li>Day {slot.weekday}, {slot.start}–{slot.end}</li>
        {/each}
      </ul>
    </section>

    <section class="surface stack" aria-labelledby="profile-area-title">
      <div class="spread">
        <h2 id="profile-area-title">Area</h2>
        <button class="link-button" type="button" onclick={() => areaSheet.open()}>Change</button>
      </div>
      <p class="mb-0">{area || 'No area set yet.'}</p>
    </section>

    <section class="surface stack" aria-labelledby="profile-settings-title">
      <div class="spread">
        <h2 id="profile-settings-title">Settings</h2>
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
    width: min(100% - 1.5rem, 42rem);
  }

  .profile-head {
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: 1rem;
  }

  .profile-head h1 {
    font-size: clamp(2rem, 8vw, 2.8rem);
  }

  .profile-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    justify-content: flex-end;
  }

  .profile-hero {
    overflow: hidden;
    border: 1px solid var(--line);
    border-radius: var(--radius);
    background: var(--surface);
    box-shadow: var(--shadow);
  }

  .hero-photo {
    position: relative;
    aspect-ratio: 4 / 3;
    max-height: 22rem;
    overflow: hidden;
    background: #dfe9df;
  }

  .hero-photo img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .hero-fallback {
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

  .hero-chip {
    position: absolute;
    left: 0.75rem;
    top: 0.75rem;
    background: rgb(255 255 255 / 92%);
  }

  .hero-body {
    display: grid;
    gap: 0.6rem;
    padding: 1.1rem;
  }

  .name-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
  }

  .hero-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    padding-top: 0.25rem;
  }

  .plain-list {
    margin: 0;
    padding-left: 1.2rem;
  }

  .photo-slot {
    position: relative;
    aspect-ratio: 1;
    overflow: hidden;
    border-radius: 20px;
    background: #e5ebe6;
  }

  .photo-slot img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .photo-placeholder {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    color: var(--muted);
    font-size: 1.25rem;
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

  @media (max-width: 600px) {
    .profile-head {
      align-items: flex-start;
      flex-direction: column;
    }

    .profile-actions {
      justify-content: flex-start;
    }
  }
</style>
