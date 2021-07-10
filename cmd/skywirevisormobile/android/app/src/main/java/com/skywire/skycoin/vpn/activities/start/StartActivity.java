package com.skywire.skycoin.vpn.activities.start;

import android.animation.Animator;
import android.animation.ObjectAnimator;
import android.os.Bundle;
import android.view.LayoutInflater;
import android.view.View;
import android.view.ViewGroup;
import android.view.animation.AccelerateInterpolator;
import android.view.animation.DecelerateInterpolator;
import android.widget.FrameLayout;

import androidx.annotation.NonNull;
import androidx.annotation.Nullable;
import androidx.fragment.app.Fragment;

import com.skywire.skycoin.vpn.R;
import com.skywire.skycoin.vpn.activities.index.IndexPageAdapter;
import com.skywire.skycoin.vpn.activities.start.connected.StartViewConnected;
import com.skywire.skycoin.vpn.activities.start.disconnected.StartViewDisconnected;
import com.skywire.skycoin.vpn.helpers.HelperFunctions;
import com.skywire.skycoin.vpn.vpn.VPNCoordinator;

import java.util.concurrent.TimeUnit;

import io.reactivex.rxjava3.android.schedulers.AndroidSchedulers;
import io.reactivex.rxjava3.core.Observable;
import io.reactivex.rxjava3.disposables.Disposable;
import io.reactivex.rxjava3.schedulers.Schedulers;

public class StartActivity extends Fragment {
    private enum SimpleVpnStates {
        Unknown,
        Running,
        Stopped,
    }

    private FrameLayout mainContainer;
    private MapBackground background;

    private StartViewDisconnected viewDisconnected;
    private StartViewConnected viewConnected;

    private SimpleVpnStates vpnState = SimpleVpnStates.Unknown;
    private ObjectAnimator animation;
    private ObjectAnimator positionAnimation;
    private SimpleVpnStates animationDestination = SimpleVpnStates.Unknown;

    private IndexPageAdapter.RequestTabListener requestTabListener;
    private Disposable serviceSubscription;

    @Nullable
    @Override
    public View onCreateView(@NonNull LayoutInflater inflater, @Nullable ViewGroup container, @Nullable Bundle savedInstanceState) {
        super.onCreateView(inflater, container, savedInstanceState);

        return inflater.inflate(R.layout.activity_start, container, true);
    }

    @Override
    public void onViewCreated(View view, Bundle savedInstanceState) {
        super.onViewCreated(view, savedInstanceState);

        mainContainer = view.findViewById(R.id.mainContainer);
        background = view.findViewById(R.id.background);

        if (!HelperFunctions.showBackgroundForVerticalScreen()) {
            background.setVisibility(View.GONE);
        }
    }

    public void setRequestTabListener(IndexPageAdapter.RequestTabListener listener) {
        requestTabListener = listener;
        if (viewDisconnected != null) {
            viewDisconnected.setRequestTabListener(listener);
        }
    }

    @Override
    public void onStart() {
        super.onStart();

        serviceSubscription = VPNCoordinator.getInstance().getEventsObservable().subscribe(state -> {
            if (state.state.val() < 10) {
                if (vpnState == SimpleVpnStates.Unknown) {
                    vpnState = SimpleVpnStates.Stopped;
                    configureViewDisconnected();
                } else {
                    vpnState = SimpleVpnStates.Stopped;
                    startInitialAnimation(SimpleVpnStates.Stopped);
                }
            } else {
                if (vpnState == SimpleVpnStates.Unknown) {
                    vpnState = SimpleVpnStates.Running;
                    configureViewConnected();
                } else {
                    vpnState = SimpleVpnStates.Running;
                    startInitialAnimation(SimpleVpnStates.Running);
                }
            }
        });
    }

    private void configureViewDisconnected() {
        if (viewDisconnected == null) {
            if (viewConnected != null) {
                mainContainer.removeView(viewConnected);
                viewConnected.close();
                viewConnected = null;
            }

            viewDisconnected = new StartViewDisconnected(getContext());
            viewDisconnected.setParentActivity(getActivity());
            if (requestTabListener != null) {
                viewDisconnected.setRequestTabListener(requestTabListener);
            }

            mainContainer.addView(viewDisconnected);
            viewDisconnected.startAnimation();
        }
    }

    private void configureViewConnected() {
        if (viewConnected == null) {
            if (viewDisconnected != null) {
                mainContainer.removeView(viewDisconnected);
                viewDisconnected.close();
                viewDisconnected = null;
            }

            viewConnected = new StartViewConnected(getContext());
            mainContainer.addView(viewConnected);
        }
    }

    private void startInitialAnimation(SimpleVpnStates desiredDestination) {
        if (animation != null || desiredDestination == SimpleVpnStates.Unknown) {
            return;
        }
        if (desiredDestination == SimpleVpnStates.Running && viewConnected != null) {
            return;
        }
        if (desiredDestination == SimpleVpnStates.Stopped && viewDisconnected != null) {
            return;
        }

        animationDestination = desiredDestination;

        View viewToAnimate;
        if (desiredDestination == SimpleVpnStates.Running) {
            viewToAnimate = viewDisconnected;
        } else {
            viewToAnimate = viewConnected;
        }

        animate(viewToAnimate, true);
    }

    private void startFinalAnimation() {
        View viewToAnimate;
        if (animationDestination == SimpleVpnStates.Running) {
            configureViewConnected();
            viewToAnimate = viewConnected;
        } else {
            configureViewDisconnected();
            viewToAnimate = viewDisconnected;
        }

        animate(viewToAnimate, false);
    }

    private void animate(View viewToAnimate, boolean isInitialAnimation) {
        if (animation != null) {
            animation.cancel();
        }
        if (positionAnimation != null) {
            positionAnimation.cancel();
        }

        float initialPosition;
        float finalPosition;
        if (animationDestination == SimpleVpnStates.Running) {
            if (isInitialAnimation) {
                initialPosition = 0;
                finalPosition = 20 * getContext().getResources().getDisplayMetrics().density;
            } else {
                initialPosition = -20 * getContext().getResources().getDisplayMetrics().density;
                finalPosition = 0;
            }
        } else {
            if (isInitialAnimation) {
                initialPosition = 0;
                finalPosition = -20 * getContext().getResources().getDisplayMetrics().density;
            } else {
                initialPosition = 20 * getContext().getResources().getDisplayMetrics().density;
                finalPosition = 0;
            }
        }

        long duration = 200;

        positionAnimation = ObjectAnimator.ofFloat(viewToAnimate, "translationY", initialPosition, finalPosition);
        positionAnimation.setDuration(duration);
        positionAnimation.setInterpolator(isInitialAnimation ? new AccelerateInterpolator() : new DecelerateInterpolator());
        positionAnimation.start();

        animation = ObjectAnimator.ofFloat(viewToAnimate, "alpha", isInitialAnimation ? 1 : 0, isInitialAnimation ? 0 : 1);
        animation.setDuration(duration);
        animation.setInterpolator(isInitialAnimation ? new AccelerateInterpolator() : new DecelerateInterpolator());

        animation.addListener(new Animator.AnimatorListener() {
            @Override
            public void onAnimationStart(Animator animation) { }
            @Override
            public void onAnimationCancel(Animator animation) { }
            @Override
            public void onAnimationRepeat(Animator animation) { }

            @Override
            public void onAnimationEnd(Animator animation) {
                if (isInitialAnimation) {
                    Observable.just(1).delay(50, TimeUnit.MILLISECONDS)
                        .subscribeOn(Schedulers.io())
                        .observeOn(AndroidSchedulers.mainThread())
                        .subscribe(v -> startFinalAnimation());
                } else {
                    finishAnimations();
                    animationDestination = SimpleVpnStates.Unknown;

                    if (vpnState == SimpleVpnStates.Running && viewConnected == null) {
                        startInitialAnimation(SimpleVpnStates.Running);
                    } else if (vpnState == SimpleVpnStates.Stopped && viewDisconnected == null) {
                        startInitialAnimation(SimpleVpnStates.Stopped);
                    }
                }
            }
        });

        animation.start();
    }

    private void finishAnimations() {
        animation.cancel();
        animation = null;

        positionAnimation.cancel();
        positionAnimation = null;
    }

    @Override
    public void onResume() {
        super.onResume();

        background.resumeAnimation();
        if (viewDisconnected != null) {
            viewDisconnected.startAnimation();
            viewDisconnected.updateRightBar();
        }
        if (viewConnected != null) {
            viewConnected.continueUpdatingStats();
            viewConnected.updateRightBar();
        }
    }

    @Override
    public void onPause() {
        super.onPause();

        background.pauseAnimation();
        if (viewDisconnected != null) {
            viewDisconnected.stopAnimation();
        }
        if (viewConnected != null) {
            viewConnected.pauseUpdatingStats();
        }
    }

    @Override
    public void onStop() {
        super.onStop();
        serviceSubscription.dispose();
    }

    @Override
    public void onDestroyView() {
        super.onDestroyView();

        background.cancelAnimation();

        if (viewDisconnected != null) {
            viewDisconnected.close();
        }
        if (viewConnected != null) {
            viewConnected.close();
        }
    }
}
