use serde::{Deserialize, Serialize};

pub const W: f32 = 1200.0;
pub const H: f32 = 750.0;
pub const PLAYER_R: f32 = 13.0;
pub const MOB_R: f32 = 9.0;
pub const DT: f32 = 1.0 / 60.0;

#[derive(Clone, Copy, Default, Serialize, Deserialize, Debug)]
pub struct Vec2 {
    pub x: f32,
    pub y: f32,
}

impl Vec2 {
    pub fn new(x: f32, y: f32) -> Self {
        Self { x, y }
    }
    pub fn distance(self, other: Self) -> f32 {
        ((self.x - other.x).powi(2) + (self.y - other.y).powi(2)).sqrt()
    }
    pub fn toward(self, other: Self) -> f32 {
        (other.y - self.y).atan2(other.x - self.x)
    }
    pub fn moved(self, angle: f32, step: f32) -> Self {
        Self::new(self.x + angle.cos() * step, self.y + angle.sin() * step)
    }
}

#[derive(Clone, Copy, Serialize, Deserialize, Debug)]
pub struct Wall {
    pub x: f32,
    pub y: f32,
    pub w: f32,
    pub h: f32,
}

pub fn circle_rect(p: Vec2, radius: f32, wall: &Wall) -> bool {
    let dx = p.x - p.x.clamp(wall.x, wall.x + wall.w);
    let dy = p.y - p.y.clamp(wall.y, wall.y + wall.h);
    dx * dx + dy * dy < radius * radius
}

pub fn blocked(p: Vec2, radius: f32, walls: &[Wall]) -> bool {
    p.x < radius
        || p.y < radius
        || p.x > W - radius
        || p.y > H - radius
        || walls.iter().any(|w| circle_rect(p, radius, w))
}

pub fn line_of_sight(a: Vec2, b: Vec2, walls: &[Wall]) -> bool {
    !walls.iter().any(|w| segment_rect(a, b, w))
}

fn segment_rect(a: Vec2, b: Vec2, r: &Wall) -> bool {
    let dx = b.x - a.x;
    let dy = b.y - a.y;
    let (mut t0, mut t1) = (0.0f32, 1.0f32);
    for (p, q) in [
        (-dx, a.x - r.x),
        (dx, r.x + r.w - a.x),
        (-dy, a.y - r.y),
        (dy, r.y + r.h - a.y),
    ] {
        if p.abs() < 1e-8 {
            if q < 0.0 {
                return false;
            }
        } else {
            let t = q / p;
            if p < 0.0 {
                if t > t1 {
                    return false;
                }
                t0 = t0.max(t);
            } else {
                if t < t0 {
                    return false;
                }
                t1 = t1.min(t);
            }
        }
    }
    true
}

pub fn segment_circle(a: Vec2, b: Vec2, center: Vec2, radius: f32) -> Option<f32> {
    let dx = b.x - a.x;
    let dy = b.y - a.y;
    let ox = a.x - center.x;
    let oy = a.y - center.y;
    let aa = dx * dx + dy * dy;
    if aa < 1e-9 {
        return (ox * ox + oy * oy <= radius * radius).then_some(0.0);
    }
    let cc = ox * ox + oy * oy - radius * radius;
    if cc <= 0.0 {
        return Some(0.0);
    }
    let bb = 2.0 * (ox * dx + oy * dy);
    let disc = bb * bb - 4.0 * aa * cc;
    if disc < 0.0 {
        return None;
    }
    let t = (-bb - disc.sqrt()) / (2.0 * aa);
    (0.0..=1.0).contains(&t).then_some(t)
}

pub fn segment_wall(a: Vec2, b: Vec2, wall: &Wall, radius: f32) -> Option<f32> {
    let expanded = Wall {
        x: wall.x - radius,
        y: wall.y - radius,
        w: wall.w + 2.0 * radius,
        h: wall.h + 2.0 * radius,
    };
    let dx = b.x - a.x;
    let dy = b.y - a.y;
    let (mut near, mut far) = (0.0f32, 1.0f32);
    for (origin, delta, min, max) in [
        (a.x, dx, expanded.x, expanded.x + expanded.w),
        (a.y, dy, expanded.y, expanded.y + expanded.h),
    ] {
        if delta.abs() < 1e-8 {
            if origin < min || origin > max {
                return None;
            }
        } else {
            let t0 = (min - origin) / delta;
            let t1 = (max - origin) / delta;
            near = near.max(t0.min(t1));
            far = far.min(t0.max(t1));
            if near > far {
                return None;
            }
        }
    }
    Some(near)
}

pub fn steer(
    position: &mut Vec2,
    facing: &mut f32,
    angle: f32,
    step: f32,
    bias: f32,
    radius: f32,
    walls: &[Wall],
) -> bool {
    for offset in [0.0, 0.6, -0.6, 1.2, -1.2, 1.9, -1.9, 2.6, -2.6] {
        let a = angle + offset * bias;
        let next = position.moved(a, step);
        if !blocked(next, radius, walls) {
            *position = next;
            *facing = a;
            return true;
        }
    }
    false
}
