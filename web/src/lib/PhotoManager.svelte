<script lang="ts">
  import { apiJSON, type Photo } from '#lib/api';
  import { online } from '#lib/offline';
  import { onMount } from 'svelte';

  export let photos: Photo[] = [];
  export let onChange: (() => void) | undefined = undefined;

  let error = '';
  let busy = false;
  let stage = '';
  let fileInput: HTMLInputElement;
  let urls: Record<string, string> = {};

  const slots = [1, 2, 3];
  const maxBytes = 5 * 1024 * 1024;
  const maxPixels = 20_000_000;

  onMount(() => {
    refresh();
    const timer = window.setInterval(refresh, 15_000);
    return () => window.clearInterval(timer);
  });

  function photoAt(order: number): Photo | undefined {
    return photos.find((photo) => photo.display_order === order);
  }

  async function refresh() {
    try {
      const result = await apiJSON<{ photos: Photo[] }>('/api/v1/photos');
      photos = result.photos;
      const next: Record<string, string> = {};
      for (const photo of photos) {
        if (photo.status === 'rejected') continue;
        try {
          const url = await apiJSON<{ url: string }>(`/api/v1/photos/${photo.photo_id}/url`);
          next[photo.photo_id] = url.url;
        } catch {
          // A missing thumbnail is not a page failure.
        }
      }
      urls = next;
      onChange?.();
    } catch {
      // A missing storage configuration is reported when an upload is attempted.
    }
  }

  async function validate(file: File): Promise<string> {
    if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
      return 'We can’t read that file. Use a JPEG, PNG or WebP photo. On iPhone: Settings › Camera › Formats › Most Compatible.';
    }
    if (file.size > maxBytes) {
      return `That photo is ${(file.size / (1024 * 1024)).toFixed(1)} MB. The limit is 5 MB. Choose a smaller one.`;
    }
    try {
      const bitmap = await createImageBitmap(file);
      const pixels = bitmap.width * bitmap.height;
      bitmap.close();
      if (pixels > maxPixels) {
        return `That photo is ${(pixels / 1_000_000).toFixed(0)} megapixels. The limit is 20. Choose a smaller version.`;
      }
    } catch {
      return 'We couldn’t read that image. Choose a different file.';
    }
    return '';
  }

  async function upload(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    error = '';
    const validation = await validate(file);
    if (validation) {
      error = validation;
      input.value = '';
      return;
    }
    busy = true;
    stage = 'Uploading…';
    try {
      const slot = await apiJSON<{ photo_id: string; upload_url: string }>(
        '/api/v1/photos/upload-url',
        { method: 'POST' }
      );
      const put = await fetch(slot.upload_url, {
        method: 'PUT',
        headers: { 'Content-Type': file.type },
        body: file
      });
      if (!put.ok) {
        throw new Error('The upload didn’t finish. Check your connection and try again.');
      }
      stage = 'Checking…';
      await apiJSON(`/api/v1/photos/${slot.photo_id}/process`, { method: 'POST' });
      await refresh();
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'We could not process that photo.';
    } finally {
      busy = false;
      stage = '';
      if (fileInput) fileInput.value = '';
    }
  }

  async function checkAgain(photo: Photo) {
    error = '';
    busy = true;
    stage = 'Checking again…';
    try {
      await apiJSON(`/api/v1/photos/${photo.photo_id}/process`, { method: 'POST' });
      await refresh();
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'We could not check that photo.';
    } finally {
      busy = false;
      stage = '';
    }
  }
</script>

<section class="surface stack" aria-labelledby="photos-title">
  <div>
    <p class="score mb-1"><span class="tennis-ball me-2" aria-hidden="true"></span>Up to 3 photos</p>
    <h2 id="photos-title">Photo check</h2>
    <p class="muted mb-0">
      JPEG, PNG or WebP, up to 5 MB and 20 MP. One approved photo puts you in Discover.
    </p>
  </div>

  <div class="slots" role="list">
    {#each slots as order}
      {@const photo = photoAt(order)}
      <div class="slot" role="listitem">
        {#if photo && photo.status !== 'rejected' && urls[photo.photo_id]}
          <img class="slot-photo" src={urls[photo.photo_id]} alt={`Photo ${order}`} />
        {:else if photo && photo.status === 'rejected'}
          <div class="slot-photo slot-status"><i class="bi bi-exclamation-triangle" aria-hidden="true"></i></div>
        {:else}
          <div class="slot-photo slot-empty"><i class="bi bi-plus-lg" aria-hidden="true"></i></div>
        {/if}
        <div class="d-flex justify-content-between align-items-center mt-2">
          <span class="score">{order === 1 ? 'Main photo' : `Photo ${order}`}</span>
          {#if photo}
            <span class:alert={photo.status === 'rejected'} class="chip">{photo.status === 'pending' ? 'Not confirmed' : photo.status === 'rejected' ? 'Not accepted' : 'Approved'}</span>
          {/if}
        </div>
        {#if photo?.status === 'pending'}
          <button class="btn btn-outline-secondary btn-sm mt-2 w-100" onclick={() => checkAgain(photo)} disabled={busy || !$online}>
            Check again
          </button>
        {/if}
      </div>
    {/each}
  </div>

  {#if error}
    <p class="alert alert-danger mb-0" role="alert">{error}</p>
  {/if}

  <label>
    <span class="btn btn-primary {busy || photos.length >= 3 || !$online ? 'disabled' : ''}">
      <i class="bi bi-camera" aria-hidden="true"></i> {busy ? stage : photos.length >= 3 ? '3 photos added' : 'Add a photo'}
    </span>
    <input
      class="visually-hidden"
      bind:this={fileInput}
      type="file"
      accept="image/jpeg,image/png,image/webp"
      onchange={upload}
      disabled={busy || photos.length >= 3 || !$online}
    />
    <small>The server validates, strips location metadata and moderates the image.</small>
  </label>
</section>

<style>
  .slots {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 0.65rem;
  }

  .slot {
    border: 1px solid var(--line);
    border-radius: 12px;
    padding: 0.5rem;
    background: var(--paper);
  }

  .slot-photo {
    width: 100%;
    aspect-ratio: 1;
    object-fit: cover;
    border-radius: 9px;
    background: #e5ebe6;
  }

  .slot-empty,
  .slot-status {
    display: grid;
    place-items: center;
    border: 1px dashed var(--control-border);
    color: var(--muted);
    font-size: 1.4rem;
  }

  .slot-status {
    background: #f7e3dd;
    color: var(--clay-deep);
    border-style: solid;
  }

  @media (max-width: 520px) {
    .slots {
      grid-template-columns: 1fr;
    }

    .slot-photo {
      aspect-ratio: 16 / 9;
    }
  }
</style>
