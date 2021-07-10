package com.skywire.skycoin.vpn.activities.start.disconnected;

import android.animation.Animator;
import android.animation.AnimatorInflater;
import android.animation.AnimatorSet;
import android.content.Context;
import android.util.AttributeSet;
import android.view.LayoutInflater;
import android.view.MotionEvent;
import android.view.View;
import android.widget.FrameLayout;
import android.widget.ImageView;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.extensible.ButtonBase;

public class StartButton extends ButtonBase implements Animator.AnimatorListener, View.OnTouchListener {
    public StartButton(Context context) {
        super(context);
    }
    public StartButton(Context context, AttributeSet attrs) {
        super(context, attrs);
    }
    public StartButton(Context context, AttributeSet attrs, int defStyle) {
        super(context, attrs, defStyle);
    }

    private FrameLayout mainLayout;
    private ImageView imageAnim;
    private ImageView imageBackground;

    private AnimatorSet animSet;

    protected void Initialize (Context context, AttributeSet attrs) {
        LayoutInflater inflater = (LayoutInflater)context.getSystemService (Context.LAYOUT_INFLATER_SERVICE);
        inflater.inflate(R.layout.view_start_button, this, true);

        mainLayout = this.findViewById(R.id.mainLayout);
        imageAnim = this.findViewById(R.id.imageAnim);
        imageBackground = this.findViewById(R.id.imageBackground);

        animSet = (AnimatorSet) AnimatorInflater.loadAnimator(getContext(), R.animator.anim_start_button);
        animSet.setTarget(imageAnim);

        setOnTouchListener(this);
        setViewForCheckingClicks(this);
    }

    public void startAnimation() {
        animSet.addListener(this);
        animSet.start();
    }

    public void stopAnimation() {
        animSet.removeAllListeners();
        animSet.cancel();
    }

    @Override
    public void onAnimationStart(Animator animation) { }
    @Override
    public void onAnimationCancel(Animator animation) { }
    @Override
    public void onAnimationRepeat(Animator animation) { }
    @Override
    public void onAnimationEnd(Animator animation) {
        animSet.start();
    }

    @Override
    public boolean onTouch(View v, MotionEvent event) {
        if (event.getAction() == MotionEvent.ACTION_DOWN) {
            mainLayout.setScaleX(0.9f);
            mainLayout.setScaleY(0.9f);
            imageBackground.setAlpha(1.0f);
        } else if (event.getAction() == MotionEvent.ACTION_CANCEL || event.getAction() == MotionEvent.ACTION_POINTER_UP || event.getAction() == MotionEvent.ACTION_UP) {
            mainLayout.setScaleX(1.0f);
            mainLayout.setScaleY(1.0f);
            imageBackground.setAlpha(0.7f);
        }

        return false;
    }

    @Override
    public void setEnabled(boolean enabled) {
        super.setEnabled(enabled);

        if (enabled) {
            setAlpha(1f);
        } else {
            setAlpha(0.5f);
        }
    }
}
