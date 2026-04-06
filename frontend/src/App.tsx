import React, { useState, useEffect, useRef } from 'react';
import videojs from 'video.js';
import 'video.js/dist/video-js.css';
import { Play, LogOut, Film, Tv, X, Settings, Folder, Plus, Trash, RefreshCw, Loader2, Search } from 'lucide-react';
import toast, { Toaster } from 'react-hot-toast';

const API_URL = `http://${window.location.hostname}:8080`;

interface Media {
  id: number;
  title: string;
  format: string;
  poster_url?: string;
  plot?: string;
  year?: number;
  rating?: number;
  type: 'movie' | 'episode' | 'series';
  season_num?: number;
  episode_num?: number;
  duration?: number;
  current_position?: number;
  genres?: string;
}

interface Series {
  id: number;
  title: string;
  poster_url?: string;
  plot?: string;
  year?: number;
  rating?: number;
  genres?: string;
}

interface Library {
  id: number;
  path: string;
  type: 'movie' | 'tv';
  last_scanned?: string;
  scanning: boolean;
  last_error?: string;
}

function App() {
  const [token, setToken] = useState<string | null>(localStorage.getItem('token'));
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [movies, setMovies] = useState<Media[]>([]);
  const [series, setSeries] = useState<Series[]>([]);
  const [continueWatching, setContinueWatching] = useState<Media[]>([]);
  const [searchResults, setSearchResults] = useState<any[]>([]);
  const [view, setView] = useState<'movies' | 'series' | 'management' | 'search'>('movies');
  const [searchQuery, setSearchQuery] = useState('');
  const [sortBy, setSortBy] = useState('newest');
  const [selectedMedia, setSelectedMedia] = useState<any | null>(null);
  const [episodes, setEpisodes] = useState<Media[]>([]);
  const [playingMedia, setPlayingMedia] = useState<Media | null>(null);
  const [isTranscoding, setIsTranscoding] = useState(false);
  const [renamePreviews, setRenamePreviews] = useState<any[]>([]);
  const [libraries, setLibraries] = useState<Library[]>([]);
  const [newLibPath, setNewLibPath] = useState('');
  const [newLibType, setNewLibType] = useState<'movie' | 'tv'>('movie');
  const videoRef = useRef<HTMLDivElement>(null);
  const playerRef = useRef<any>(null);

  useEffect(() => {
    if (token) {
      fetchData();
    }
  }, [token, view, sortBy]);

  const fetchData = async () => {
    const headers = { Authorization: `Bearer ${token}` };
    try {
      const cwRes = await fetch(`${API_URL}/api/continue-watching`, { headers });
      const cwData = await cwRes.json();
      setContinueWatching(cwData || []);

      if (view === 'movies') {
        const res = await fetch(`${API_URL}/api/media?sort=${sortBy}`, { headers });
        const data = await res.json();
        setMovies(data || []);
      } else if (view === 'series') {
        const res = await fetch(`${API_URL}/api/series?sort=${sortBy}`, { headers });
        const data = await res.json();
        setSeries(data || []);
      }
    } catch (err) {
      toast.error("Failed to fetch library data");
    }
  };

  const handleSearch = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    if (!searchQuery) return;
    setView('search');
    try {
      const res = await fetch(`${API_URL}/api/search?q=${encodeURIComponent(searchQuery)}`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      const data = await res.json();
      setSearchResults(data || []);
    } catch (err) {
      toast.error("Search failed");
    }
  };

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    const res = await fetch(`${API_URL}/api/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password })
    });
    if (res.ok) {
      const data = await res.json();
      setToken(data.token);
      localStorage.setItem('token', data.token);
      toast.success("Welcome to DEX");
    } else {
      toast.error("Invalid credentials");
    }
  };

  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault();
    const res = await fetch(`${API_URL}/api/register`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password })
    });
    if (res.ok) {
      toast.success("Registered successfully! You can now login.");
    } else {
      toast.error("Registration failed. Username may be taken.");
    }
  };

  const openDetails = async (item: any) => {
    setSelectedMedia(item);
    if (item.type === 'series' || (view === 'series' && !item.type)) {
      const res = await fetch(`${API_URL}/api/series/${item.id}/episodes`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      const data = await res.json();
      setEpisodes(data || []);
    }
  };

  const startPlayback = async (media: Media) => {
    const loadingToast = toast.loading("Starting transcoder...");
    setIsTranscoding(true);
    setPlayingMedia(media);
    setSelectedMedia(null);
    
    try {
      const res = await fetch(`${API_URL}/api/stream/${media.id}/start`, {
        headers: { Authorization: `Bearer ${token}` }
      });

      if (res.status === 202 || res.ok) {
        setTimeout(() => {
          setIsTranscoding(false);
          toast.dismiss(loadingToast);
        }, 2000);
      } else {
        throw new Error();
      }
    } catch (err) {
      toast.error("Failed to start stream", { id: loadingToast });
      setIsTranscoding(false);
      setPlayingMedia(null);
    }
  };

  useEffect(() => {
    if (playingMedia && !isTranscoding && videoRef.current && !playerRef.current) {
      const videoElement = document.createElement('video-js');
      videoElement.classList.add('vjs-big-play-centered');
      videoRef.current.appendChild(videoElement);

      playerRef.current = videojs(videoElement, {
        autoplay: true,
        controls: true,
        responsive: true,
        fluid: true,
        sources: [{
          src: `${API_URL}/api/stream/${playingMedia.id}/index.m3u8`,
          type: 'application/x-mpegURL'
        }]
      });

      fetch(`${API_URL}/api/media/${playingMedia.id}/subtitles`, {
        headers: { Authorization: `Bearer ${token}` }
      })
      .then(res => res.json())
      .then((subs: string[]) => {
        subs.forEach(subFile => {
          playerRef.current.addRemoteTextTrack({
            kind: 'subtitles',
            label: subFile.replace(/\.[^/.]+$/, ""),
            srclang: 'en',
            src: `${API_URL}/api/media/${playingMedia.id}/subtitles/${subFile}`
          }, false);
        });
      });

      playerRef.current.ready(() => {
        if (playingMedia.current_position) {
          playerRef.current.currentTime(playingMedia.current_position);
        }
      });

      const interval = setInterval(() => {
        if (playerRef.current && !playerRef.current.paused()) {
          const currentTime = Math.floor(playerRef.current.currentTime());
          const duration = Math.floor(playerRef.current.duration());
          if (currentTime > 0) {
            fetch(`${API_URL}/api/media/${playingMedia.id}/progress`, {
              method: 'POST',
              headers: { 
                'Authorization': `Bearer ${token}`,
                'Content-Type': 'application/json'
              },
              body: JSON.stringify({ current_position: currentTime, duration })
            });
          }
        }
      }, 10000);

      return () => {
        clearInterval(interval);
        if (playerRef.current) {
          playerRef.current.dispose();
          playerRef.current = null;
        }
      };
    }
  }, [playingMedia, isTranscoding]);

  const fetchLibraries = async () => {
    const res = await fetch(`${API_URL}/api/libraries`, {
      headers: { Authorization: `Bearer ${token}` }
    });
    const data = await res.json();
    setLibraries(data || []);
  };

  const addLibrary = async () => {
    if (!newLibPath) return;
    const res = await fetch(`${API_URL}/api/libraries`, {
      method: 'POST',
      headers: { 
        'Authorization': `Bearer ${token}`,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ path: newLibPath, type: newLibType })
    });
    if (res.ok) {
      toast.success("Library added");
      setNewLibPath('');
      fetchLibraries();
    } else {
      toast.error("Failed to add library");
    }
  };

  const removeLibrary = async (id: number) => {
    await fetch(`${API_URL}/api/libraries/${id}`, {
      method: 'DELETE',
      headers: { Authorization: `Bearer ${token}` }
    });
    toast.success("Library removed");
    fetchLibraries();
  };

  const scanLibrary = async (id: number) => {
    await fetch(`${API_URL}/api/libraries/${id}/scan`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}` }
    });
    toast.success("Scan started");
    fetchLibraries();
  };

  const fetchRenamePreviews = async () => {
    const res = await fetch(`${API_URL}/api/renamer/preview`, {
      headers: { Authorization: `Bearer ${token}` }
    });
    const data = await res.json();
    setRenamePreviews(data || []);
  };

  const applyRenames = async () => {
    const toRename = renamePreviews.filter(p => p.needs_rename);
    if (toRename.length === 0) {
      toast.error("Nothing to rename");
      return;
    }
    const res = await fetch(`${API_URL}/api/renamer/apply`, {
      method: 'POST',
      headers: { 
        'Authorization': `Bearer ${token}`,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(toRename.map(p => ({ id: p.id, new_path: p.new_path })))
    });
    if (res.ok) {
      toast.success("Renames applied successfully");
      fetchRenamePreviews();
    } else {
      toast.error("Failed to apply renames");
    }
  };

  useEffect(() => {
    let interval: any;
    if (token && view === 'management') {
      fetchLibraries();
      fetchRenamePreviews();
      interval = setInterval(fetchLibraries, 5000);
    }
    return () => clearInterval(interval);
  }, [token, view]);

  if (!token) {
    return (
      <div style={{ background: '#111', color: '#fff', height: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', fontFamily: 'sans-serif' }}>
        <Toaster position="top-center" />
        <div style={{ background: '#222', padding: '3rem', borderRadius: '12px', width: '300px' }}>
          <h1 style={{ textAlign: 'center', marginBottom: '2rem' }}>DEX</h1>
          <form onSubmit={handleLogin} style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
            <input placeholder="Username" style={{ padding: '0.8rem', borderRadius: '4px', border: 'none' }} value={username} onChange={e => setUsername(e.target.value)} />
            <input type="password" placeholder="Password" style={{ padding: '0.8rem', borderRadius: '4px', border: 'none' }} value={password} onChange={e => setPassword(e.target.value)} />
            <button type="submit" style={{ padding: '0.8rem', background: '#e50914', color: '#fff', border: 'none', borderRadius: '4px', fontWeight: 'bold', cursor: 'pointer' }}>Login</button>
            <button type="button" onClick={handleRegister} style={{ background: 'transparent', color: '#aaa', border: 'none', cursor: 'pointer' }}>Register</button>
          </form>
        </div>
      </div>
    );
  }

  return (
    <div style={{ background: '#111', color: '#fff', minHeight: '100vh', fontFamily: 'sans-serif' }}>
      <Toaster position="top-center" />
      <nav style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '1rem 2rem', background: 'rgba(0,0,0,0.8)', position: 'sticky', top: 0, zIndex: 100 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '2rem' }}>
          <h1 style={{ color: '#e50914', margin: 0, letterSpacing: '2px', cursor: 'pointer' }} onClick={() => setView('movies')}>DEX</h1>
          <div style={{ display: 'flex', gap: '1rem' }}>
            <button onClick={() => setView('movies')} style={{ background: 'none', border: 'none', color: view === 'movies' ? '#fff' : '#aaa', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
              <Film size={20} /> Movies
            </button>
            <button onClick={() => setView('series')} style={{ background: 'none', border: 'none', color: view === 'series' ? '#fff' : '#aaa', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
              <Tv size={20} /> TV Shows
            </button>
            <button onClick={() => setView('management')} style={{ background: 'none', border: 'none', color: view === 'management' ? '#fff' : '#aaa', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
              <Settings size={20} /> Management
            </button>
          </div>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '1.5rem' }}>
          <form onSubmit={handleSearch} style={{ position: 'relative' }}>
            <Search size={18} style={{ position: 'absolute', left: '0.8rem', top: '50%', transform: 'translateY(-50%)', color: '#aaa' }} />
            <input 
              placeholder="Search..." 
              style={{ background: '#333', color: '#fff', border: 'none', padding: '0.6rem 1rem 0.6rem 2.5rem', borderRadius: '20px', width: '250px' }} 
              value={searchQuery}
              onChange={e => setSearchQuery(e.target.value)}
            />
          </form>
          <button onClick={() => { setToken(null); localStorage.removeItem('token'); }} style={{ background: 'none', border: 'none', color: '#aaa', cursor: 'pointer' }}>
            <LogOut size={20} />
          </button>
        </div>
      </nav>

      <main style={{ padding: '2rem' }}>
        {(view === 'movies' || view === 'series') && (
          <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: '1.5rem' }}>
            <select 
              style={{ background: '#333', color: '#fff', border: 'none', padding: '0.5rem', borderRadius: '4px' }}
              value={sortBy}
              onChange={e => setSortBy(e.target.value)}
            >
              <option value="newest">Recently Added</option>
              <option value="rating">Highest Rated</option>
              <option value="year">Release Year</option>
            </select>
          </div>
        )}

        {view === 'management' ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '3rem' }}>
            <section>
              <h2 style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '1.5rem' }}><Folder size={24} /> Library Locations</h2>
              <div style={{ background: '#222', padding: '1.5rem', borderRadius: '8px', marginBottom: '1.5rem' }}>
                <h3>Add New Library</h3>
                <div style={{ display: 'flex', gap: '1rem', marginTop: '1rem' }}>
                  <input placeholder="Path" style={{ flex: 1, padding: '0.8rem', borderRadius: '4px', border: 'none', background: '#333', color: '#fff' }} value={newLibPath} onChange={e => setNewLibPath(e.target.value)} />
                  <select style={{ padding: '0.8rem', borderRadius: '4px', border: 'none', background: '#333', color: '#fff' }} value={newLibType} onChange={e => setNewLibType(e.target.value as any)}>
                    <option value="movie">Movies</option>
                    <option value="tv">TV Shows</option>
                  </select>
                  <button onClick={addLibrary} style={{ background: '#e50914', color: '#fff', border: 'none', padding: '0.8rem 1.5rem', borderRadius: '4px', fontWeight: 'bold', cursor: 'pointer' }}><Plus size={20} /></button>
                </div>
              </div>
              <div style={{ background: '#222', borderRadius: '8px', overflow: 'hidden' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left' }}>
                  <thead style={{ background: '#333' }}>
                    <tr><th style={{ padding: '1rem' }}>Path</th><th style={{ padding: '1rem' }}>Type</th><th style={{ padding: '1rem' }}>Status</th><th style={{ padding: '1rem' }}>Actions</th></tr>
                  </thead>
                  <tbody>
                    {libraries.map(lib => (
                      <tr key={lib.id} style={{ borderBottom: '1px solid #333' }}>
                        <td style={{ padding: '1rem' }}>{lib.path}</td>
                        <td style={{ padding: '1rem', textTransform: 'capitalize' }}>{lib.type}</td>
                        <td style={{ padding: '1rem' }}>
                          {lib.scanning ? (
                            <span style={{ color: '#e50914', display: 'flex', alignItems: 'center', gap: '0.5rem' }}><RefreshCw className="animate-spin" size={16} /> Scanning...</span>
                          ) : lib.last_error ? (
                            <span style={{ color: '#ff9800' }} title={lib.last_error}>Error</span>
                          ) : (
                            <span style={{ color: '#aaa' }}>{lib.last_scanned ? new Date(lib.last_scanned).toLocaleString() : 'Never'}</span>
                          )}
                        </td>
                        <td style={{ padding: '1rem' }}>
                          <button onClick={() => scanLibrary(lib.id)} disabled={lib.scanning} style={{ marginRight: '0.5rem', background: 'none', border: 'none', color: lib.scanning ? '#555' : '#fff', cursor: lib.scanning ? 'default' : 'pointer' }}><RefreshCw size={16} /></button>
                          <button onClick={() => removeLibrary(lib.id)} style={{ background: 'none', border: 'none', color: '#e50914', cursor: 'pointer' }}><Trash size={16} /></button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>

            <section>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.5rem' }}>
                <h2 style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}><Settings size={24} /> File Renamer</h2>
                <button onClick={applyRenames} style={{ background: '#e50914', color: '#fff', border: 'none', padding: '0.8rem 1.5rem', borderRadius: '4px', fontWeight: 'bold', cursor: 'pointer' }}>Apply All</button>
              </div>
              <div style={{ background: '#222', borderRadius: '8px', overflow: 'hidden' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left' }}>
                  <thead style={{ background: '#333' }}>
                    <tr><th style={{ padding: '1rem' }}>Current</th><th style={{ padding: '1rem' }}>Suggested</th></tr>
                  </thead>
                  <tbody>
                    {renamePreviews.map(p => (
                      <tr key={p.id} style={{ borderBottom: '1px solid #333' }}>
                        <td style={{ padding: '1rem', fontSize: '0.8rem', color: '#aaa' }}>{p.old_path.split('/').pop()}</td>
                        <td style={{ padding: '1rem', color: p.needs_rename ? '#fff' : '#4caf50' }}>{p.new_name}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
          </div>
        ) : view === 'search' ? (
          <section>
            <h2 style={{ marginBottom: '2rem' }}>Search Results for "{searchQuery}"</h2>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))', gap: '1.5rem' }}>
              {searchResults.map((item: any) => (
                <div key={`${item.type}-${item.id}`} style={{ cursor: 'pointer' }} onClick={() => openDetails(item)}>
                  <img src={item.poster_url || 'https://via.placeholder.com/500x750?text=No+Poster'} style={{ width: '100%', borderRadius: '8px' }} />
                  <h3 style={{ fontSize: '0.9rem', marginTop: '0.5rem' }}>{item.title}</h3>
                  <div style={{ fontSize: '0.8rem', color: '#aaa' }}>{item.type === 'movie' ? 'Movie' : 'Series'} • {item.year}</div>
                </div>
              ))}
            </div>
            {searchResults.length === 0 && <p>No results found.</p>}
          </section>
        ) : (
          <>
            {continueWatching.length > 0 && (
              <section style={{ marginBottom: '3rem' }}>
                <h2 style={{ marginBottom: '1.5rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}><RefreshCw size={24} /> Continue Watching</h2>
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: '1.5rem' }}>
                  {continueWatching.map(item => (
                    <div key={item.id} style={{ cursor: 'pointer' }} onClick={() => startPlayback(item)}>
                      <div style={{ position: 'relative', borderRadius: '8px', overflow: 'hidden', aspectRatio: '16/9' }}>
                        <img src={item.poster_url || 'https://via.placeholder.com/500x750?text=No+Poster'} style={{ width: '100%', height: '100%', objectFit: 'cover' }} />
                        {item.current_position && item.duration && (
                          <div style={{ position: 'absolute', bottom: 0, left: 0, right: 0, height: '4px', background: 'rgba(255,255,255,0.3)' }}>
                            <div style={{ width: `${(item.current_position / item.duration) * 100}%`, height: '100%', background: '#e50914' }} />
                          </div>
                        )}
                      </div>
                      <h3 style={{ fontSize: '0.9rem', marginTop: '0.5rem' }}>{item.title}</h3>
                    </div>
                  ))}
                </div>
              </section>
            )}

            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))', gap: '1.5rem' }}>
              {(view === 'movies' ? movies : series).map((item: any) => (
                <div key={item.id} style={{ cursor: 'pointer' }} onClick={() => openDetails(item)}>
                  <img src={item.poster_url || 'https://via.placeholder.com/500x750?text=No+Poster'} style={{ width: '100%', borderRadius: '8px' }} />
                  <h3 style={{ fontSize: '0.9rem', marginTop: '0.5rem' }}>{item.title}</h3>
                  <div style={{ fontSize: '0.8rem', color: '#aaa' }}>{item.year} • ★{item.rating?.toFixed(1) || 'N/A'}</div>
                </div>
              ))}
            </div>
          </>
        )}
      </main>

      {selectedMedia && (
        <div style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.8)', zIndex: 200, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <div style={{ background: '#222', width: '90%', maxWidth: '900px', borderRadius: '12px', padding: '2rem', position: 'relative' }}>
            <button onClick={() => setSelectedMedia(null)} style={{ position: 'absolute', top: '1rem', right: '1rem', background: 'none', border: 'none', color: '#fff', cursor: 'pointer' }}><X /></button>
            <div style={{ display: 'flex', gap: '2rem' }}>
              <img src={selectedMedia.poster_url || 'https://via.placeholder.com/500x750?text=No+Poster'} style={{ width: '250px', borderRadius: '8px' }} />
              <div>
                <h1 style={{ margin: 0 }}>{selectedMedia.title}</h1>
                <div style={{ color: '#e50914', fontWeight: 'bold', margin: '0.5rem 0' }}>{selectedMedia.genres}</div>
                <div style={{ marginBottom: '1rem', color: '#aaa' }}>{selectedMedia.year} • ★{selectedMedia.rating?.toFixed(1)}</div>
                <p style={{ color: '#ddd', lineHeight: '1.6' }}>{selectedMedia.plot}</p>
                {view === 'movies' || selectedMedia.type === 'movie' ? (
                  <button onClick={() => startPlayback(selectedMedia as Media)} style={{ background: '#fff', color: '#000', padding: '0.8rem 1.5rem', borderRadius: '4px', fontWeight: 'bold', marginTop: '1rem', cursor: 'pointer', border: 'none' }}>
                    {(selectedMedia as Media).current_position ? 'Resume' : 'Play'}
                  </button>
                ) : (
                  <div style={{ marginTop: '1.5rem' }}>
                    <h3>Episodes</h3>
                    <div style={{ maxHeight: '250px', overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
                      {episodes.map(ep => (
                        <div key={ep.id} onClick={() => startPlayback(ep)} style={{ padding: '0.8rem', background: '#333', borderRadius: '4px', cursor: 'pointer', display: 'flex', justifyContent: 'space-between' }}>
                          <div style={{ display: 'flex', flexDirection: 'column' }}>
                            <span>S{ep.season_num}E{ep.episode_num} - {ep.title}</span>
                            {ep.current_position && ep.duration && (
                              <div style={{ width: '100px', height: '4px', background: '#555', marginTop: '4px' }}>
                                <div style={{ width: `${(ep.current_position / ep.duration) * 100}%`, height: '100%', background: '#e50914' }} />
                              </div>
                            )}
                          </div>
                          <Play size={16} />
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      )}

      {(playingMedia || isTranscoding) && (
        <div style={{ position: 'fixed', inset: 0, background: '#000', zIndex: 300, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <button onClick={() => { setPlayingMedia(null); setIsTranscoding(false); fetchData(); }} style={{ position: 'absolute', top: '1rem', right: '1rem', zIndex: 310, color: '#fff', background: 'rgba(255,255,255,0.1)', border: 'none', padding: '0.5rem', borderRadius: '4px', cursor: 'pointer' }}>Close</button>
          {isTranscoding ? (
            <div style={{ textAlign: 'center' }}>
              <Loader2 className="animate-spin" size={48} style={{ marginBottom: '1rem' }} />
              <p>Starting Transcoder...</p>
            </div>
          ) : (
            <div ref={videoRef} style={{ width: '100%', maxWidth: '1200px' }} />
          )}
        </div>
      )}
    </div>
  );
}

export default App;
