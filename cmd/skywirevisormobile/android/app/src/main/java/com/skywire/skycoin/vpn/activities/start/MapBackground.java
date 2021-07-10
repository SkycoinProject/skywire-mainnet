package com.skywire.skycoin.vpn.activities.start;

import android.animation.Animator;
import android.animation.ObjectAnimator;
import android.content.Context;
import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.graphics.Canvas;
import android.graphics.Rect;
import android.graphics.drawable.BitmapDrawable;
import android.util.AttributeSet;
import android.view.View;
import android.view.animation.AccelerateInterpolator;
import android.view.animation.DecelerateInterpolator;

import com.skywire.skycoin.vpn.R;

public class MapBackground extends View {
    public MapBackground(Context context) {
        super(context);
        Initialize(context, null);
    }
    public MapBackground(Context context, AttributeSet attrs) {
        super(context, attrs);
        Initialize(context, attrs);
    }
    public MapBackground(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
        Initialize(context, attrs);
    }

    private BitmapDrawable bitmapDrawable;
    private float proportion = 1;
    private Rect drawableArea = new Rect(0, 0,1, 1);
    private int widthSize;
    private boolean finished = false;
    private ObjectAnimator animation;

    private void Initialize (Context context, AttributeSet attrs) {
        Bitmap bitmap = BitmapFactory.decodeResource(getResources(), R.drawable.map_phones);
        bitmapDrawable = new BitmapDrawable(context.getResources(), bitmap);
        bitmapDrawable.setAlpha(25);

        proportion = (float)bitmap.getWidth() / (float)bitmap.getHeight();
    }

    public void pauseAnimation() {
        if (animation != null) {
            animation.pause();
        }
    }

    public void resumeAnimation() {
        if (animation != null) {
            animation.resume();
        }
    }

    public void cancelAnimation() {
        finished = true;
        stopAnimation();
    }

    @Override
    protected void onMeasure(int widthMeasureSpec, int heightMeasureSpec) {
        widthSize = MeasureSpec.getSize(widthMeasureSpec);
        int heightSize = MeasureSpec.getSize(heightMeasureSpec);

        if (widthSize != drawableArea.width() || heightSize != drawableArea.height()) {
            setValues(widthSize, heightSize);
        }

        setMeasuredDimension(drawableArea.width(), drawableArea.height());
    }

    @Override
    protected void onDraw(Canvas canvas) {
        bitmapDrawable.draw(canvas);
        super.onDraw(canvas);
    }

    private void setValues(int width, int height) {
        if (finished) {
            return;
        }

        drawableArea = new Rect(0, 0, (int) (height * proportion), height);
        bitmapDrawable.setBounds(drawableArea);

        stopAnimation();
        selectPosition();
        startAnimation(true);
    }

    private void selectPosition() {
        int max = drawableArea.width() - widthSize;
        this.setTranslationX(-(int)Math.round(Math.random() * max));
        invalidate();
    }

    private void startAnimation(boolean appear) {
        animation = ObjectAnimator.ofFloat(this, "alpha", appear ? 0 : 1, appear ? 1 : 0);
        animation.setDuration(800);
        animation.setInterpolator(appear ? new DecelerateInterpolator() : new AccelerateInterpolator());
        if (!appear) {
            animation.setStartDelay(15000);
        }

        animation.addListener(new Animator.AnimatorListener() {
            @Override
            public void onAnimationStart(Animator animation) { }
            @Override
            public void onAnimationCancel(Animator animation) { }
            @Override
            public void onAnimationRepeat(Animator animation) { }

            @Override
            public void onAnimationEnd(Animator anim) {
                stopAnimation();
                if (appear) {
                    startAnimation(false);
                } else {
                    selectPosition();
                    startAnimation(true);
                }
            }
        });

        animation.start();
    }

    private void stopAnimation() {
        if (animation != null) {
            animation.removeAllListeners();
            animation.cancel();
            animation = null;
        }
    }
}
